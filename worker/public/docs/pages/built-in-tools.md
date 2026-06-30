# Built-in Tools

## File operations

| Tool | Description |
|---|---|
| `read_file` | Read a text, PDF, DOCX, PPTX, CSV/TSV, or image file. Must be called before `patch_file`. **Sensitive file guard**: SSH keys, `.pem`, `.key`, `.env` always require confirmation regardless of sudo or allowlist |
| `write_file` | Create or fully overwrite a file |
| `patch_file` | Precise string replacement (supports `replace_all`) |
| `list_files` | List a directory's contents |
| `glob_files` | Glob pattern search |
| `search_files` | Regex search inside file contents |

## Web (read-only, mostly concurrent)

| Tool | Concurrent | Description |
|---|---|---|
| `fetch_page` | ✓ | Fetch a web page (readability + 4xx/5xx skip cache via ToriiDB); `save=true` persists to local file |
| `search_web` | | DuckDuckGo lite endpoint, package-level rate limit (2 s gap) |
| `search_google_news` | ✓ | Google News RSS |

## HTTP

| Tool | Concurrent | Description |
|---|---|---|
| `send_http_request` | ✓ | Raw HTTP request, returns status + headers + body. GET is auto-allowed; other methods require confirmation. Built-in SSRF guard (DNS-resolved against loopback / private / link-local); bypass specific hosts via `config.json` `net_white_list` |
| `download_file` | ✓ | Download a binary file to local disk (tar.gz, images, archives); for JSON/HTML use `send_http_request` or `fetch_page` |

## Media

| Tool | Concurrent | Description |
|---|---|---|
| `transcribe_media` | ✓ | Local audio / video transcription via Gemini `inline_data` (ogg, mp3, wav, m4a, flac, aac, mp4, mov, webm, mpeg, 3gp); 20 MiB / request cap. *(gemini credential needed)* |
| `generate_image` | | Generate an image via gpt-image-2 on the codex@ subscription quota. Confirm size and quality via `ask_user` first. Output as `[SEND_FILE:<path>]`. 15 min cap. *(codex credential needed)* |

## Utility

| Tool | Concurrent | Description |
|---|---|---|
| `calculate` | ✓ | Math expression evaluator |

## Agent orchestration

| Tool | Description |
|---|---|
| `invoke_subagent` | In-process subagent (no HTTP); supports `name` / `session_id` / `model` / `system_prompt` / `exclude_tools`. Forced-exclude set: `invoke_subagent` itself, `invoke_external_agent`, `cross_review_with_external_agents`, `review_result`. `AllowAll` and `WorkDir` inherit from parent ctx |
| `invoke_external_agent` | One-shot external CLI (claude / codex / copilot / gemini); `readonly` flag controls write permission. Subprocess timeout capped by `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` (default 10 min) |
| `cross_review_with_external_agents` | Chain four external CLIs through up to three review rounds (`MaxVerifyRounds=3`, package const). 15 min hard cap |
| `review_result` | Internal priority-model self-review |
| `generate_plan` | Returns a structured markdown plan (requirement summary / prerequisites / steps + acceptance / overall acceptance / risks / fallback). Uses `exec.SelectAgent` with `[plan]` prefix to trigger P0.6 routing for strong reasoning agent. `toolDefs=nil` — plan only, no execution. 5 min cap |

## Interactive

| Tool | Description |
|---|---|
| `ask_user` | Free-text / single-select / multi-select / `secret` masked input prompts; routes through `pending` registry when active, else falls back to stdin (CLI) or non-interactive guidance |
| `store_secret` | Captures a value via masked input and writes directly to keychain — **the value never enters the LLM context, history, or logs**. Schema does **not** accept a `value` parameter; the agent only sees `name` + description |
| `install_dependence` | Install a missing system binary cross-platform (TUI/CLI only). Skips if already in PATH. Sandbox blocks sudo, so this tool bypasses it. Language-level packages (pip/npm/cargo/gem) → output command for user to run manually |

## Memory

| Tool | Description |
|---|---|
| `search_chat_history` | Keyword + semantic dual-track search over the current session's history |
| `remember_error` | Record a tool error with resolution / strategy |
| `search_error_history` | Cross-session semantic search over error memory |
| `read_error` | Read a specific error entry by key |

## Diagnostics

| Tool | Description |
|---|---|
| `read_log` | Return recent WARN/ERROR lines from daemon.log (last `h` hours) |
| `report_error` | Scan daemon.log for WARN/ERROR lines and upload to report.agenvoy.com. Fire-and-forget |

## RAG

External-document RAG via the KuraDB child process. Tools are **per-turn dynamically excluded** when `~/.config/kuradb/endpoint` is absent — the LLM never sees them when KuraDB is off.

| Tool | Description |
|---|---|
| `list_rag` | List available KuraDB databases (e.g. `notes`, `inbox`, `code`) |
| `search_rag` | Search a database by keyword (`mode=keyword`, `gse` tokenization, Chinese-aware) or semantic (`mode=semantic`, OpenAI `text-embedding-3-small`) |

When `list_rag` / `search_rag` tools are loaded, the system prompt forces the **first wave** of tool calls for any information query to be `list_rag` + `search_rag`. External web/search tools become secondary (gap-filling), not fallback or substitute.

## Render

| Tool | Description |
|---|---|
| `render_page` | Overwrite the rendered HTML page for the current session canvas; browser tabs auto-reload via SSE |

## Channel

Cross-session push tools and channel format references. Each tool gates on both `cfg.{T,D}Enabled` and keychain credential presence.

| Tool | Description |
|---|---|
| `list_chatbot` | List authorized chats for the specified platform (`platform=telegram` or `platform=discord`). *(telegram or discord needed)* |
| `send_to_chatbot` | Send a formatted message to an authorized chat by `target_id`. Requires `platform` param. Telegram: HTML + transient client. Discord: markdown + transient client. *(telegram or discord needed)* |
| `format_chatbot` | `AlwaysLoad=true`; returns the full formatting reference for the specified platform (Telegram HTML or Discord markdown). *(telegram or discord needed)* |

## Output markers (channel-specific behavior)

Output text from any tool or LLM response is post-processed for these markers:

| Marker | Behavior |
|---|---|
| `[SEND_FILE:<path>]` | Channel runtime auto-attaches the file (Telegram → photo/document split by ext, Discord → unified `SendFiles` batched 10/msg) |
| `[SEND_VOICE:<text>]` | Telegram only. Synthesizes via Gemini TTS, sends as OGG voice. Run.go fires the upload **async** (`go func` with `context.WithoutCancel`); reply text returns immediately. Failure → `slog.Error` + chat notify (never silent) |

Marker regex + dedupe + `os.Stat` filtering lives in `internal/utils/utils.go`. Telegram-specific photo/document split wrapper in `internal/runtime/telegram/fileMarker.go`. Push hooks (`telegram.PushTelegramResult` / `discord.PushDiscordResult`) call the same extractor.

## Skill discovery

| Tool | Description |
|---|---|
| `run_skill` | Load a skill into the current loop (synthesizes a tool_call/tool_result pair into ToolHistories) |
| `search_tools` | Search the registered tool catalog |
| `list_tools` | List all registered tools |

## Skill & tool variants (always-allowed `write_file` variants)

| Tool | Description |
|---|---|
| `write_skill` | Create or rewrite a file under `~/.config/agenvoy/skills/` |
| `patch_skill` | String replacement inside a skill file |
| `remove_skill` | Move a skill directory to `.Trash/` |
| `write_tool` | Create or overwrite `tool.json` or `script.py` under `~/.config/agenvoy/tools/script/` |
| `patch_tool` | String replacement inside a script tool file (`tool.json` or `script.py`) |
| `test_tool` | Run a script tool's `script.py` with JSON input inside sandbox |
| `remove_tool` | Move a script tool directory to `.Trash/` |

All variants are always-allowed and scoped to their respective directories. Every write/patch/remove auto-commits to the corresponding git repo (skills or tools). `write_tool` and `write_skill` support concurrent calls.

## Git versioning & self-improvement

| Tool | Description |
|---|---|
| `git_log` | List git commit history for skills or tools directory (`tag` = `skills` or `tools`) |
| `git_rollback` | Roll back skills or tools directory to a specified git commit (`tag` = `skills` or `tools`) |

**Self-improvement loop**: when a skill execution produces tool errors (wrong tool name, failed steps), `postSkillImprove` runs synchronously at the end of `Execute`. It loads the built-in `improve-skill` definition, feeds it the execution trace, rewrites the faulty SKILL.md/scripts, and auto-commits the fix.

## System

| Tool | Description |
|---|---|
| `run_command` | Execute a system command (argv-only schema, sandbox-wrapped via `go-pkg/sandbox`); `cd` is special-cased and mutates `Executor.WorkDir` directly without going through the sandbox |

## Scheduler

| Tool | Description |
|---|---|
| `add_schedule` | Bind an existing scheduler skill to a one-shot fire time (`target=task`) or a 5-field cron expression (`target=cron`). Task time formats: `+5m` (relative), `HH:MM` (today), `YYYY-MM-DD HH:MM`, or RFC3339. **Must be invoked by `scheduler-skill-creator` skill, not directly** — direct call requires the skill to already exist under `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`. |
| `patch_schedule` | Reschedule by `skill_name` and `target`; changes only the time, leaves the bound SKILL body untouched. |
| `remove_schedule` | Cancel by `skill_name` and `target`; the bound scheduler skill dir is moved to `.Trash/`. |
| `list_schedule` | List tasks and/or crons in current session. `target` accepts `task`, `cron`, or `all` (default). |

`scheduler-skill-creator` is the high-level skill that **creates** a scheduler skill body and calls `add_schedule` to bind it. New recurring / one-shot requests should activate that skill, not call the low-level tools directly.

The daemon-side runtime (`internal/runtime/scheduler.go`) watches `~/.config/agenvoy/{tasks,crons}.json` with fsnotify and hot-reloads on Write / Create / Rename. Past-due tasks are auto-fired and removed on startup or reload; fire executes via `runtime.SetRunner` → in-process subagent over the scheduler skill body (always-allow context).

TUI surfaces three slash commands for managing schedules: `/cron`, `/task` (add / remove / edit), and `/sched-<name>` (manual trigger of an existing scheduler skill body).
