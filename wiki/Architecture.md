# Architecture

> [中文](https://github.com/agenvoy/Agenvoy/wiki/架構)

A high-level view of how Agenvoy fits together. For per-module diagrams, sequence flows, and the tool-dispatch state machine, jump into the topic-specific pages linked at the bottom of this page.

## Overview

```mermaid
graph TB
    subgraph Entry["Entry · cmd/app"]
        App["agen model · session · mcp · stop · update<br/>make app · TUI + Discord + REST<br/>make cli / run · single-shot"]
    end

    subgraph Engine["Engine · internal/agents/exec"]
        Run["exec.Run<br/>:bot → MatchExternal → MatchSkillCall"]
        Execute["exec.Execute · ≤128 iterations<br/>3-pass parallel tool calls"]
        Sub["ExecWithSubagent<br/>in-process · ctx inheritance"]
    end

    subgraph Pending["Pending · internal/pending"]
        Reg["Global confirm/ask registry<br/>per-entry buffered=1 reply ch"]
    end

    subgraph Providers["LLM Providers · 7"]
        P["Claude · OpenAI · Codex · Gemini<br/>Copilot · Nvidia · Compat"]
    end

    subgraph Tools["Tool Subsystem"]
        T["File · Web · Git · API · Script<br/>activate_skill · invoke_subagent<br/>ask_user · scheduler · memory"]
        MCP["MCP Adapter<br/>stdio · HTTP/SSE"]
        Ext["External CLI<br/>codex · claude · copilot · gemini"]
    end

    subgraph Security["Security · go-pkg/sandbox"]
        S["bwrap (Linux) / sandbox-exec (macOS)<br/>filesystem.Policy · DeniedMap"]
    end

    subgraph Session["Session · internal/session"]
        SL["bot.md · status.json · action.log<br/>observer · fsnotify watch"]
    end

    subgraph Memory["Memory · ToriiDB"]
        M["DBSessionHist · DBSessionSummary<br/>error_memory · 90d TTL<br/>OpenAI text-embedding-3-small"]
    end

    App --> Run
    Run --> Execute
    Execute -->|invoke_subagent| Sub
    Sub --> Execute
    Execute -->|Send| Providers
    Execute -->|tool calls| Tools
    Tools --> MCP
    Tools --> Ext
    Tools --> Security
    Execute <-->|confirm/ask| Pending
    Sub <-->|subagent ask_user| Pending
    Execute <--> Memory
    Execute <--> Session
```

## Layers

| Layer | Package | Responsibility |
|---|---|---|
| Entry | `cmd/app` | argv dispatch (`model` / `session` / `mcp` / `cli` / `run` / `stop` / `update`); init env, sandbox, filesystem policy, MCP manager |
| Runtime singleton | `internal/runtime` | server-mode UID lock; SIGTERM prior server on startup |
| Engine | `internal/agents/exec` | iteration loop; tool dispatch; provider routing |
| Subagent | `internal/agents/subagent` | in-process child agent (no HTTP) |
| External agents | `internal/agents/external` | one-shot subprocess wrappers (codex / claude / copilot / gemini) |
| Providers | `internal/agents/provider/<name>` | unified `Agent.Send()` interface |
| Tools | `internal/tools` + adapters | built-in / API / script / MCP tool definitions |
| Sandbox | `go-pkg/sandbox` | OS-native isolation, single entry `Wrap()` |
| Filesystem | `go-pkg/filesystem` (+ `reader/`) + `internal/filesystem` | policy-aware writes; ToriiDB pathing |
| Session | `internal/session` | bot.md / status.json / action.log / fsnotify observer |
| Pending | `internal/pending` | global confirm/ask registry |
| Memory | ToriiDB (`DBSessionHist` / `DBSessionSummary` / `error_memory`) | semantic search + 90-day TTL |
| Scheduler | `internal/scheduler` (+ TUI watcher) | cron / one-shot tasks; hot-reload on file change |
| TUI | `internal/tui` | bubbletea inline-chat front-end; single-package by design |

## Cross-cutting principles

- **OS-native sandbox over Go-side filters** — security policy is enforced at the OS boundary; new restrictions go into `go-pkg/sandbox`, not into agenvoy callers
- **Prompt as policy** — permission mode, sensitive operations, and system-prompt protection live in `configs/prompts/`; adding a category means editing the prompt, not the engine
- **In-process over HTTP for subagents** — `invoke_subagent` calls `exec.Execute` directly, sharing the same provider clients, sandbox, pending registry, and memory layer; `AllowAll` and `WorkDir` flow through ctx
- **Read tools fan out, write tools serialize** — concurrency is opt-in and requires both "no side effects" and "upstream allows parallelism"
- **One config layer per concern** — providers in `configs/jsons/providors/`, MCP in `mcp.json`, persona in `bot.md`; each tool author / user touches at most one file
- **Single source of truth per artifact** — `~/.claude/CLAUDE.md` mirrors to the global Obsidian vault; skills mirror between `~/.claude/skills/` and `extensions/skills/`

## TUI design choices

> Per pardn chiu: *"bubbletea isn't designed to be split into separate modules that reference each other — splitting it would make the lifecycle a mess. I don't have the bandwidth to handle it right now."* This module is intentionally kept undivided.

The TUI lives in a single package (`internal/tui`) and is **not** split into subpackages. Every file under `internal/tui/` follows this principle.

### Why bubbletea (not tview / tcell)

The previous TUI used `rivo/tview` (archived under `internal/_tui_archived/`). It was replaced because:

- **Inline scrollback**: bubbletea's `tea.Println` writes lines that scroll into the terminal's native buffer above the input box. tview owns the entire screen and can't co-exist with shell scrollback.
- **lipgloss styling primitives**: borders, padding, foreground/background composition compose cleanly. tview styles are tag-based and harder to reuse across components.
- **bubbles ecosystem**: `textarea`, `spinner`, `cursor` are drop-in components that match the rest of the charm-bracelet style.

The cost is that bubbletea is a Go port of [The Elm Architecture](https://guide.elm-lang.org/architecture/) — its `tea.Model` interface is monolithic by design.

### Why a single package

`tea.Model` requires `Update(tea.Msg) (tea.Model, tea.Cmd)` to be a method on the model type. Methods must live in the same package as the type. This forces:

- All `Update` logic in the same package as the model
- Splitting into subpackages requires a wrapper in a third (root) package, plus exporting **every** model field so the sub-packages can read/write state
- Currently `unexported` types like `popupState`, `commandPickerState`, `viewMode` would have to become exported, creating an "API" that no one outside `internal/tui` will ever consume
- `send()` and `program atomic.Pointer[tea.Program]` either move into a sub-package (root sets via setter API) or stay in root and force handlers to import root, which creates a second cycle

A real Go-style TUI would build per-domain widget packages (each owning its state struct, render method, and event handler) with bubbletea acting only as event loop. That refactor is a 600–800 LOC rewrite split into 4 phases. For the current ~1.1k LOC TUI maintained by one developer, the gain doesn't justify the cost.

### When to revisit

Switch to per-domain widget packages when **any one** of:

- TUI exceeds ~3k LOC and code review keeps stalling on "where does this belong"
- Multiple developers regularly touch the TUI and step on each other's state
- Specific widgets need independent unit tests against frozen state — currently impossible without instantiating the whole `Model`

## Where to read more

| Topic | Page |
|---|---|
| Iteration loop, three-pass dispatch in detail | [Core Concepts](https://github.com/agenvoy/Agenvoy/wiki/Core-Concepts) |
| Provider routing and planner | [Providers](https://github.com/agenvoy/Agenvoy/wiki/Providers) |
| Tool registry, extension paths | [Tools](https://github.com/agenvoy/Agenvoy/wiki/Tools) |
| Memory tiers and semantic search | [Memory System](https://github.com/agenvoy/Agenvoy/wiki/Memory-System) |
| Sandbox policy, permission modes | [Security and Sandbox](https://github.com/agenvoy/Agenvoy/wiki/Security-and-Sandbox) |
| MCP transports, lifecycle | [MCP Integration](https://github.com/agenvoy/Agenvoy/wiki/MCP-Integration) |
| Source of truth for architecture rules | [CLAUDE.md](https://github.com/pardnchiu/agenvoy/blob/main/CLAUDE.md) |
