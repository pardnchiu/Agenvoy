# Core Concepts

> [中文](https://github.com/agenvoy/Agenvoy/wiki/核心概念)

## Session

A session is the core unit in Agenvoy. Each session has its own conversation context, memory, agent persona, and tool configuration.

Storage path: `~/.config/agenvoy/sessions/<sid>/`

| File | Purpose |
|---|---|
| `bot.md` | Agent persona definition (YAML frontmatter + markdown body) |
| `status.json` | Current execution state and active task list |
| `action.log` | Tool call audit log; rotates at 1 MB (truncates to 768 KB) |
| `mcp.json` | Session-scoped MCP server configuration |

History, summaries, and config flags live in ToriiDB (`DBSessionHist`, `DBSessionSummary`, `DBConfig`) rather than per-session JSON.

### Session prefixes and lifetime

| Prefix | Lifetime |
|---|---|
| `cli-*` | Permanent (created by `agen session new` / `make cli`) |
| `http-*` | Permanent (created by `POST /v1/send` with `persist=true`) |
| `dc-*` | Permanent (Discord channels) |
| `tg-*` | Permanent (Telegram chats — per-chat, shared across users in that chat) |
| `temp-*` | Reaped after 1 h idle (default for `POST /v1/send`) |
| `temp-sub-*` | Reaped after 1 h idle (subagent default) |

`runApp` startup runs `CleanupSessions()` against the `temp-*` whitelist only — `cli-*`, `http-*`, `dc-*`, and `tg-*` are never auto-reaped.

## bot.md — Agent Persona

Each session can declare its own persona:

```markdown
---
name: mobile-builder
---

You are an expert mobile application architect specializing in
SwiftUI, Jetpack Compose, and React Native...
```

The frontmatter `name` doubles as a lookup key (`GetSessionIDByName`); the body is rendered into the system prompt's `## Bot Persona` block on each turn. `agen session config` opens the current session's bot.md in `$EDITOR`.

## Agent routing

Three ways decide which agent handles a task:

**1. Automatic** — A dispatcher LLM analyzes the input and picks the best-fit provider via `SelectAgent()`.

**2. `:name` one-shot override** (CLI / TUI) — Prefix any input with `:session-name` to dispatch one command at the named session **without** changing the primary pointer:

```
:mobile-builder build me a SwiftUI login screen
```

Resolution order in `exec.Run`: `:bot` → `MatchExternal` (`/claude` etc.) → `MatchSkillCall` (`/skill-name`) → `Execute`. The `:name` override is parsed in `exec.Run` (CLI/TUI) and in the Telegram runtime (which strips the prefix and falls back with a metadata note if the name is not found); HTTP `POST /v1/send` and Discord do not interpret the prefix.

**3. `invoke_subagent` tool** — An agent calls another agent in-process (no HTTP) during execution, inheriting `AllowAll` and `WorkDir` from the parent ctx. The forced-exclude set is `{invoke_subagent, invoke_external_agent, cross_review_with_external_agents, review_result}`; `ask_user` is **not** excluded — subagents can ask the user via the shared pending registry.

## Iteration loop

Each request runs the main loop in `exec.Execute()` for up to **128 iterations**. Each iteration:

1. Assemble messages: `SystemPrompts` + `OldHistories` + `UserInput` + `ToolHistories`
2. Call `Agent.Send()` on the chosen provider
3. Parse `tool_calls` from the response
4. Dispatch the tool calls through `toolCall.go` (three-pass concurrency, see below)
5. Append results to `ToolHistories`
6. Stop when no `tool_calls` remain or the iteration limit is hit

There is **no inter-round delay** — rate-limit protection comes from provider round-trip latency, the same-error circuit breaker, and per-tool internal limiters (e.g., `search_web` 2 s gap, `api_*` per-name 1 s gap).

## Three-pass tool concurrency

`toolCall.go` splits each round's tool calls into three serial passes; only Pass 2 fans out:

| Pass | Mode | Work |
|---|---|---|
| 1 — pre-flight | Serial | Cache hit check (skipped for `read_file`), stub-tool short-circuit, confirm gate, JSON-schema validation |
| 2 — execute | Concurrent for `IsConcurrent`-tagged tools; serial otherwise | `tools.Execute` |
| 3 — commit | Serial | Land `sessionData.Tools` and `ToolHistories`, update cache, emit `EventToolResult`, handle review tools |

Concurrent-tagged tools: `fetch_page`, `invoke_subagent`, `calculator`, `send_http_request`, `fetch_google_rss`, `fetch_yahoo_finance`, `fetch_youtube_transcript`, `transcribe_media`. `search_web`, write-class tools, `api_*`, and MCP tools always run serially.

## Pending registry

`internal/runtime/pending.go` is the prefix-routed confirm/ask listener registry shared by the main agent and any in-process subagents. Producers (`toolCall` confirm, `ask_user` handler, `store_secret` handler) call `Ask(ctx, req)` and block on a per-entry buffered=1 reply channel; each runtime registers a listener via `pending.RegisterListener(prefix)` (TUI/CLI use `""` to match all, the Telegram daemon listener uses `"tg-"`) and only claims matching entries through `PickNextFor(prefix)`. ctx cancellation removes the entry so a stale producer never wastes a human interaction.

The gate `pending.HasListener(sessionID)` checks whether a listener with a matching prefix is registered for that session. This replaces the old global `pending.Active atomic.Bool` so Telegram, Discord, and CLI confirm flows can run side by side without blocking each other.

## Circuit breaker

When `Agent.Send()` returns the same error signature three times in a row (e.g., HTTP 429 with identical request payload), the loop aborts to prevent infinite retry storms. Distinct error signatures reset the counter.

## Permission mode

| Mode | Behavior |
|---|---|
| `single-confirm` | Each non-ReadOnly tool call requires user confirmation (default for `agen cli`) |
| `always-allow` | Tools auto-execute; the LLM is instructed to invoke `ask_user` first for seven categories of truly irreversible operations |

The seven irreversible categories that still require explicit `ask_user` under `always-allow`:

1. `rm -rf` on populated directories
2. `DROP TABLE` / `DROP DATABASE`
3. `git push --force` to `main`
4. `chmod 777` on system paths
5. Overwriting a non-empty file that has not been read
6. Cloud resource deletion
7. `shutdown` / `kill -9` on system processes

The gate is enforced by the system prompt, not by hardcoded Go-side filters — adding a new category means editing `configs/prompts/` only.

## Per-session concurrency

`MAX_SESSION_TASKS` (default `3`, hard cap `10`) limits how many concurrent `Execute()` calls a single session can run. Excess callers wait via `EnterConcurrent(sid)` and only appear in `status.json` once a slot is free.

## Cross-turn workdir reset

Each new user message rebuilds the `Executor` and resets `data.WorkDir` to the process cwd via `os.Getwd()` — `cd`-mutated workdir does **not** persist across turns. Two guardrails prevent the LLM from inferring stale workdir from history:

- **L1 (system prompt)** — `Work directory: {{.WorkPath}}` line plus an explicit reminder that prior `cd` text is from older turns
- **L2 (per-message)** — Every user message is wrapped with `---\n當前時間: ...\n工作目錄: <data.WorkDir>\n---\n<input>`; the workDir line is the strongest anchor and overrides any history recency bias

The TUI strips the wrapper visually via `stripUserMetaHeader`; the LLM still receives it verbatim.
