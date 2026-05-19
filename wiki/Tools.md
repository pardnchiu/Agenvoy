# Tools

> [中文](https://github.com/agenvoy/Agenvoy/wiki/工具系統)

## Built-in tools

### File operations

| Tool | Description |
|---|---|
| `read_file` | Read a file (text, PDF via `pdftotext`, PPTX slide-level, DOCX line-level, CSV/TSV → JSON 2D array, image extension errors back to `read_image`) |
| `read_image` | Read an image as base64 + metadata for vision-capable models |
| `write_file` | Create or fully overwrite a file |
| `patch_file` | Precise string replacement (supports `replace_all`) |
| `list_files` | List a directory's contents |
| `glob_files` | Glob pattern search |
| `search_content` | Regex search inside file contents |

### Web (read-only, mostly concurrent)

| Tool | Concurrent | Description |
|---|---|---|
| `fetch_page` | ✓ | Fetch a web page (readability + 4xx/5xx skip cache via ToriiDB) |
| `save_page_to_file` | | Fetch and persist to local file |
| `search_web` | | DuckDuckGo lite endpoint, package-level rate limit (2 s gap) |
| `fetch_google_rss` | ✓ | Google News RSS |
| `fetch_yahoo_finance` | ✓ | Stock and financial data |
| `fetch_youtube_transcript` | ✓ | YouTube subtitle fetch (Gemini-backed) |
| `transcribe_media` | ✓ | Local audio / video transcription via Gemini `inline_data` (ogg, mp3, wav, m4a, flac, aac, mp4, mov, webm, mpeg, 3gp); 20 MiB / request cap aligned with Telegram `Bot.Save` |
| `send_http_request` | ✓ | Raw HTTP request, returns status + headers + body |
| `calculator` | ✓ | Math expression evaluator |

### Agent orchestration

| Tool | Description |
|---|---|
| `invoke_subagent` | In-process subagent (no HTTP); supports `name` / `session_id` / `model` / `system_prompt` / `exclude_tools` |
| `invoke_external_agent` | One-shot external CLI (claude / codex / copilot / gemini); `readonly` flag controls write permission |
| `cross_review_with_external_agents` | Chain four external CLIs through up to three review rounds (`MaxVerifyRounds=3`, package const) |
| `review_result` | Internal priority-model self-review |
| `ask_user` | Free-text / single-select / multi-select / `secret` masked input prompts; routes through `pending` registry when active, else falls back to stdin (CLI) or non-interactive guidance |
| `store_secret` | Captures a value via masked input and writes directly to keychain — **the value never enters the LLM context, history, or logs** |

### Memory

| Tool | Description |
|---|---|
| `search_conversation_history` | Keyword + semantic dual-track search over the current session's history |
| `remember_error` | Record a tool error with resolution / strategy |
| `search_error_memory` | Cross-session semantic search over error memory |
| `read_error_memory` | Read a specific error entry by key |

### Skill discovery

| Tool | Description |
|---|---|
| `activate_skill` | Load a skill into the current loop (synthesizes a tool_call/tool_result pair into ToolHistories) |
| `search_tools` | Search the registered tool catalog |
| `list_tools` | List all registered tools |

### System

| Tool | Description |
|---|---|
| `run_command` | Execute a system command (argv-only schema, sandbox-wrapped via `go-pkg/sandbox`); `cd` is special-cased and mutates `Executor.WorkDir` directly without going through the sandbox |

### Scheduler

| Tool | Description |
|---|---|
| `add_task` / `add_cron` | Bind an existing scheduler skill to a one-shot fire time or a 5-field cron expression. `add_task` time formats: `+5m` (relative), `HH:MM` (today), `YYYY-MM-DD HH:MM`, or RFC3339. **Must be invoked by `scheduler-skill-creator` skill, not directly** — direct call requires the skill to already exist under `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`. |
| `patch_task` / `patch_cron` | Reschedule by `skill_name`; changes only the time, leaves the bound SKILL body untouched. |
| `remove_task` / `remove_cron` | Cancel by `skill_name`; the bound scheduler skill dir is moved to `.Trash/`. |

`scheduler-skill-creator` is the high-level skill that **creates** a scheduler skill body and calls `add_task` / `add_cron` to bind it. New recurring / one-shot requests should activate that skill, not call the low-level tools directly.

The daemon-side runtime (`internal/runtime/scheduler.go`) watches `~/.config/agenvoy/{tasks,crons}.json` with fsnotify and hot-reloads on Write / Create / Rename. Past-due tasks are auto-fired and removed on startup or reload; fire executes via `runtime.SetRunner` → in-process subagent over the scheduler skill body (always-allow context).

TUI surfaces three slash commands for managing schedules: `/cron`, `/task` (add / remove / edit), and `/sched-<name>` (manual trigger of an existing scheduler skill body). See [CLI Reference](CLI-Reference) for popup flows.

## Tool extension

### Script tools (`script_*`)

Drop a Python / Node.js / shell script under `extensions/scripts/<name>/` along with a `tool.json` descriptor. Agenvoy auto-registers it as `script_<name>` at startup.

```
extensions/scripts/my-tool/
├── tool.json     # name, description, parameter schema, command
└── run.py        # actual script
```

### API tools (`api_*`)

Drop a JSON file under `extensions/apis/<name>.json` describing a REST endpoint. It auto-registers as `api_<name>`. Each `api_<name>` has its own per-name 1 s rate limiter (`reserveAPISlot`).

> **Confirm gate** — `api_*` tools are **not** prefix-exempt from confirmation. Users may define destructive endpoints (DELETE / POST writes), so `agen cli` confirms each call. Use `agen run` for batch auto-approval.

### MCP tools (`mcp__*`)

Tools exposed by an MCP server are auto-registered as `mcp__<server>__<tool>`. See [MCP Integration](https://github.com/agenvoy/Agenvoy/wiki/MCP-Integration) for configuration. MCP tool output is capped at **1 MiB** per call to keep tool results within provider limits.

## Tool design rules

The four mandatory rules for adding or editing tools (enforced by `/tool-reviewer`):

1. **Name is the only semantic carrier** — stub-tool first calls only see the name; description and params arrive on the second round
2. **Description serves parameter-call correctness only** — no usage manuals, trigger conditions, or comparisons with other tools
3. **English only** — Chinese only appears in user-facing handler return messages
4. **Optional fields must declare a `default`** — handlers still defend against nil/missing

Description length: a single verb-led sentence by default. Forbidden: trigger conditions ("Use when ..."), tool comparisons, downstream flow instructions, output schema details.

## Tool concurrency markers

Tools have two independent flags:

- `ReadOnly` — exempts from confirm gate when `agen cli` is in use
- `Concurrent` — opts into Pass 2 fan-out (parallel goroutine per call)

Adding `Concurrent: true` requires both "no side effects" and "upstream allows parallelism". The current concurrent set is documented in [Core Concepts](https://github.com/agenvoy/Agenvoy/wiki/Core-Concepts#three-pass-tool-concurrency).

## Credential auto-heal

`store_secret` is `AlwaysLoad: true` so the agent sees it on the first round. When a downstream tool returns a missing-key or invalid-credential error (`401` / `403` / `invalid api key` / `expired token`), the system prompt's `§10 Credential auto-heal` SOP directs the agent to call `store_secret` (which captures the new value through masked input — the value never reaches the LLM) and retry the original tool. Capped at two `store_secret` rounds per failing tool per turn.
