# Tools

> [中文](Tools.zh.md)

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
| `transcribe_media` | ✓ | Local audio / video transcription via Gemini `inline_data` (ogg, mp3, wav, m4a, flac, aac, mp4, mov, webm, mpeg, 3gp); 20 MiB / request cap aligned with Telegram `Bot.Save` |
| `send_http_request` | ✓ | Raw HTTP request, returns status + headers + body |
| `calculator` | ✓ | Math expression evaluator |

### Agent orchestration

| Tool | Description |
|---|---|
| `invoke_subagent` | In-process subagent (no HTTP); supports `name` / `session_id` / `model` / `system_prompt` / `exclude_tools`. Forced-exclude set: `invoke_subagent` itself, `invoke_external_agent`, `cross_review_with_external_agents`, `review_result`. `AllowAll` and `WorkDir` inherit from parent ctx |
| `invoke_external_agent` | One-shot external CLI (claude / codex / copilot / gemini); `readonly` flag controls write permission. Subprocess timeout capped by `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` (default 10 min) |
| `cross_review_with_external_agents` | Chain four external CLIs through up to three review rounds (`MaxVerifyRounds=3`, package const). 15 min hard cap |
| `review_result` | Internal priority-model self-review |
| `generate_plan` | Returns a structured markdown plan (requirement summary / prerequisites / steps + acceptance / overall acceptance / risks / fallback). Uses `exec.SelectAgent(ctx, dispatcher, registry, "[plan] " + requirement, ...)` — the `[plan]` prefix triggers `agent_selector.md` P0.6 routing to pick a strong reasoning agent (claude-opus > codex-pro > codex > claude-sonnet > ...). Sends the agent with `toolDefs=nil` so the planner has no tools to call — plan only, no execution. 5 min cap |
| `ask_user` | Free-text / single-select / multi-select / `secret` masked input prompts; routes through `pending` registry when active, else falls back to stdin (CLI) or non-interactive guidance |
| `store_secret` | Captures a value via masked input and writes directly to keychain — **the value never enters the LLM context, history, or logs**. Schema does **not** accept a `value` parameter; the agent only sees `name` + description |

### Memory

| Tool | Description |
|---|---|
| `search_conversation_history` | Keyword + semantic dual-track search over the current session's history |
| `remember_error` | Record a tool error with resolution / strategy |
| `search_error_memory` | Cross-session semantic search over error memory |
| `read_error_memory` | Read a specific error entry by key |

### RAG

External-document RAG via the KuraDB child process. See [KuraDB RAG](KuraDB-RAG.md) for lifecycle / health check. Tools are **per-turn dynamically excluded** when `~/.config/kuradb/endpoint` is absent — the LLM never sees them when KuraDB is off.

| Tool | Description |
|---|---|
| `rag_list_db` | List available KuraDB databases (e.g. `notes`, `inbox`, `code`) |
| `rag_search_keyword` | Keyword search a database via `gse` tokenization (Chinese-aware) |
| `rag_search_semantic` | Semantic search a database via OpenAI embeddings (`text-embedding-3-small`) |

When `rag_*` tools are loaded, the system prompt forces the **first wave** of tool calls for any information query to be `rag_list_db` + `rag_search_*`. External web/search tools become secondary (gap-filling), not fallback or substitute.

### Render

| Tool | Description |
|---|---|
| `update_page` | Overwrite the rendered HTML page for the current session canvas; browser tabs auto-reload via SSE |

### Channel

Cross-session push tools and channel format references. Each tool gates on both `cfg.{T,D}Enabled` and keychain credential presence.

| Tool | Description |
|---|---|
| `list_telegram_chat` | List authorized Telegram chats (`id` + `name`); reads `~/.config/agenvoy/.telegram`. *(telegram needed)* |
| `send_to_telegram_chat` | Send an HTML-formatted message to an authorized chat by `chat_id`. Transient client (not the daemon's long-polling bot). Forced `parse_mode=HTML`. *(telegram needed)* |
| `telegram_format` | `AlwaysLoad=true`; returns the full Telegram HTML formatting reference (allowed tags, escape rules, file/voice markers, char limits). *(telegram needed)* |
| `list_discord_channel` | List authorized Discord channels (`id` + `name`). *(discord needed)* |
| `send_to_discord_channel` | Send a markdown-formatted message to an authorized channel by `channel_id`. Transient client via daemon REST. *(discord needed)* |
| `discord_format` | `AlwaysLoad=true`; returns Discord markdown reference. *(discord needed)* |

### Output markers (channel-specific behavior)

Output text from any tool or LLM response is post-processed for these markers:

| Marker | Behavior |
|---|---|
| `FILE: <path>` (full-line, any line) | Channel runtime auto-attaches the file (Telegram → photo/document split by ext, Discord → unified `SendFiles` batched 10/msg) |
| `[SEND_FILE:<path>]` (inline) | Same as above — LLM emits this when proactively wanting to attach |
| `[SEND_VOICE:<text>]` | Telegram only. Synthesizes via Gemini TTS, sends as OGG voice. Run.go fires the upload **async** (`go func` with `context.WithoutCancel`); reply text returns immediately. Failure → `slog.Error` + chat notify `⚠️ SendVoice failed (background)` (never silent) |

Marker regex + dedupe + `os.Stat` filtering lives in `internal/utils/fileMarker.go`. Push hooks (`telegram.PushTelegramResult` / `discord.PushDiscordResult`) call the same extractor.

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

Tools exposed by an MCP server are auto-registered as `mcp__<server>__<tool>`. See [MCP Integration](MCP-Integration.md) for configuration. MCP tool output is capped at **1 MiB** per call to keep tool results within provider limits.

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

Adding `Concurrent: true` requires both "no side effects" and "upstream allows parallelism". The current concurrent set is documented in [Core Concepts](Core-Concepts.md#three-pass-tool-concurrency).

## Tool timeout matrix

Each adapter has its own timeout, layered with the executor-side ceiling:

| Adapter | Default | Configurable | Where |
|---|---|---|---|
| Built-in (`toolRegister.Dispatch`) | 1 min | `Def.Timeout` per tool | tool registration |
| Script (`script_*`) | 5 min (300s) | `tool.json` `"timeout": <seconds>` | `extensions/scripts/<name>/tool.json` |
| API (`api_*`) | 60s | `doc.Endpoint.Timeout`; hard cap 300s | `extensions/apis/<name>.json` |
| MCP HTTP | 60s `http.Client.Timeout` + 1 min outer dispatch | n/a | MCP server config |
| MCP stdio | 1 min outer dispatch only | n/a | MCP server config |

Long-running tools (script + API) emit `running name=... elapsed=Ys/Zs` to the daemon log every 30s for visibility.

Subagent + external-agent tools have their own multi-minute caps (`invoke_subagent` = `MAX_SUBAGENT_TIMEOUT_MIN`, `invoke_external_agent` = 10 min, `cross_review_with_external_agents` = 15 min, `generate_plan` / `transcribe_media` = 5 min).

## Credential auto-heal

`store_secret` is `AlwaysLoad: true` so the agent sees it on the first round. When a downstream tool returns a missing-key or invalid-credential error (`401` / `403` / `invalid api key` / `expired token`), the system prompt's `§10 Credential auto-heal` SOP directs the agent to call `store_secret` (which captures the new value through masked input — the value never reaches the LLM) and retry the original tool. Capped at two `store_secret` rounds per failing tool per turn.
