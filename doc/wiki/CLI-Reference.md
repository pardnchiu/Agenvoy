# CLI Reference

> [中文](CLI-Reference.zh.md)

## Top-level dispatch

`agen` parses `os.Args[1]` and dispatches to one of the subcommands below; running with no subcommand attaches the TUI (and fork-execs a daemon if none is running).

```bash
agen                                           # Attach TUI; spawn daemon if not running
agen model   {add|remove|list|dispatcher|reasoning}
agen session {new|switch|config} [name]
agen mcp     {list|add|remove}
agen cli     <input...>                        # one-shot, requires per-tool confirm
agen run     <input...>                        # one-shot, auto-approves all tools
agen stop                                      # Stop the running daemon
agen update                                    # Download latest release & rebuild
```

### `agen model`

| Subcommand | Action |
|---|---|
| `add` | Interactive provider/model add (writes credentials to keychain) |
| `remove` (alias `rm`) | Interactive provider/model remove |
| `list` | List registered models |
| `dispatcher` | Choose the dispatcher model |
| `reasoning` | Set dispatcher reasoning effort: `low` / `medium` / `high` / `xhigh` |

### `agen session`

| Subcommand | Action |
|---|---|
| `new <name>` | Create a `cli-<uuid>` session, write its `bot.md` (frontmatter `name=<name>`), switch primary pointer |
| `switch [name]` | Switch primary pointer; without `name`, an interactive picker opens with the current session highlighted (Enter = stay) |
| `config [name]` | Open the target session's `bot.md` in `$EDITOR`; without `name`, picker opens |

### `agen mcp`

| Subcommand | Action |
|---|---|
| `list` | List all configured MCP servers (global + per-session) |
| `add` | Interactive add — name → transport (Local stdio / Remote HTTP) → fields → scope (Global / pick a session) |
| `remove` | Interactive remove with scope label |

### `agen stop`

SIGTERM the running daemon (5 s grace period, then SIGKILL); clears `~/.config/agenvoy/runtime.uid`. Prints `No daemon running.` and exits 0 if no daemon is alive.

### `agen update`

Always-overwrite update to the latest release. Downloads `https://cloud.agenvoy.com/update.sh` to a `/tmp/agenvoy-update-*.sh` file, executes it via `bash`, and removes the temp file on completion (SIGINT/SIGTERM also cleaned). The script clones the latest tag into `mktemp -d "${TMPDIR:-/tmp}/agenvoy-update.XXXXXX"`, runs `make build`, and prints a summary box pointing to `agen` for the next launch. Daemon keeps the old inode after replacement — run `agen stop` and re-attach to pick up the new build.

## `make` shortcuts

```bash
make build                      # Compile and install to /usr/local/bin/agen
make app                        # Full stack (TUI + Discord + Telegram + REST API)
make stop                       # Stop the running daemon
make update                     # = agen update
make cli <input...>             # agen cli <input...>
make run <input...>             # agen run <input...>
make model   [add|remove|list|dispatcher|reasoning]
make session [new|switch|config] [name]
make mcp     [list|add|remove]
```

## TUI shortcuts

Single bubbletea textarea (`internal/runtime/tui`); slash commands open transient popups that close cleanly back to the prompt.

| Key | Action |
|---|---|
| `Ctrl+S` | Submit current textarea content (Enter inserts a newline; `Alt+Enter` also inserts newline) |
| `/` | Begin slash-command filter (popup picker — Up / Down to navigate, Tab / Enter to autocomplete into the textarea, Esc to dismiss) |
| `Up` / `Down` (on empty / single-line input) | Walk input history (per-session `input_history` file) |
| `Esc` | Cancel running exec (if running) or dismiss the active popup |
| `Ctrl+C` | Exit TUI (daemon keeps running) |

The TUI auto-tails the active session's `action.log` (foreign-process writes prefixed with `▌ ` in warn-purple). Single-session view only — multi-session dashboard archived.

## TUI slash commands

| Command | Description |
|---|---|
| `/switch` | Pick a session (current pre-selected; `(new session)` sentinel at the bottom). |
| `/new [name]` | Create a session; optional name pins it to the registry (conflict-checked). |
| `/bot [name body...]` | Edit bot persona — two-popup form (name then multiline body), or inline `parts≥3` for fast path. |
| `/model [global\|session\|dispatch\|summary\|reasoning]` | `global` → add / remove from registry, `session` → pick from `cfg.Models`, `dispatch` → set dispatcher model, `summary` → set summary model (or `(use dispatcher)` to fall back), `reasoning` → set reasoning depth (`low` / `medium` / `high`). |
| `/mcp [add\|remove]` | Chained popup form for MCP server config; restart daemon to apply. |
| `/feature [voice\|image2\|kuradb]` | `voice` → enable / disable voice message handling, `image2` → enable / disable gpt-image-2 generation, `kuradb` → toggle KuraDB RAG (see [KuraDB RAG](KuraDB-RAG.md)). |
| `/discord [enable\|disable]` | Toggle Discord bot connection (in-TUI popup chain: token entry → verification → keychain write → daemon fsnotify reload). |
| `/telegram [enable\|disable]` | Toggle Telegram bot connection (same in-TUI popup chain as `/discord`; first chat to message the bot must pass an in-chat 6-digit OTP, then chat ID is appended to `~/.config/agenvoy/.telegram`). |
| `/cron [add\|remove\|edit]` | Recurring schedules. `add` → multiline requirement → dispatches `/scheduler-skill-creator <requirement>` (skill asks for missing when/what via `ask_user`). `remove` → list → confirm → `runtime.RemoveCron` + trashes skill dir. `edit` → list → requirement → agent picks `patch_schedule(target=cron)` or rewrites SKILL body. Picker is **session-scoped** — only shows entries with `session_id == currentSessionID`. |
| `/task [add\|remove\|edit]` | One-shot tasks (mirrors `/cron`; uses `add_schedule` / `patch_schedule` / `remove_schedule` with `target=task`). Session-scoped picker. |
| `/sched-<name>` | Surfaced in the slash picker after regular skills (warn-purple label) — picks an existing scheduler skill and dispatches its body with an explicit "execute, do NOT activate scheduler-skill-creator" preamble. Filtered by session — only skills bound to the current session's task/cron entries appear. |
| `/dangerous [remove-session\|allow-skill\|allow-cmd\|allow-report]` | `remove-session` → delete current session (double-confirm), `allow-skill` → mark skill as always-allow (bypasses confirm gate), `allow-cmd` → append binary to `white_list`, `allow-report` → enable / disable error report upload. |
| `/history` | Reload visible transcript — clear screen, reprint header, render last 100 entries from session `action.log`. |
| `/log` | Open raw `action.log` in `$PAGER` (fallback `less -Rf +G`, jumps to bottom). `\x1F` markers expanded to newlines for readability. |
| `/cmd` | Run a shell command directly in the current workDir (`sh -c`). |
| `/update` | Confirm → `agen stop && agen update` via `tea.ExecProcess` → quit (re-attach with `agen` to pick up the new binary). |
| `/clear` | Clear terminal display only — memory untouched. |
| `/exit`, `/quit` | Exit TUI. |

## Auto mode

Press **Shift+Tab** to toggle auto mode. The current mode is shown at the bottom-left of the TUI:

- `[safe]` (default) — tool calls require user confirmation before execution
- `[auto]` — all tool calls are automatically approved (`allowAll = true`); sandbox and validator still apply

Auto mode is session-local and resets when the TUI restarts. It can also be set at launch via `agen --allow-all`.

## Input prefixes

Resolution order in `exec.Run()` (CLI / TUI / Telegram only — Discord and HTTP do not parse `:name`):

1. **`:name`** — session override (one-shot routing without changing primary pointer)
2. **`MatchExternal`** — external CLI agent dispatch (`/claude`, `/codex`, etc.)
3. **`MatchSkillCall`** — skill activation (`/<skill-name>`)

### `:name` session override

```bash
make cli ":ship-v0.20 /commit-generate"
```

Composable with skills and external agents — order resolves left to right (`:bot` → external → skill → execute).

### External CLI prefixes

| Prefix | Mode | Underlying flags |
|---|---|---|
| `/claude` | Read-only | `claude -p --disallowedTools=Edit,Write,NotebookEdit` |
| `/claude-allow` | Write | `claude -p --permission-mode acceptEdits` |
| `/codex` | Read-only | codex CLI (default sandbox) + `--output-last-message` + `--skip-git-repo-check` |
| `/codex-allow` | Write | codex CLI `--dangerously-bypass-approvals-and-sandbox` |
| `/gh` or `/copilot` | Read-only | `gh copilot -s` (no write variant exists) |
| `/gemini` | Read-only | `gemini --approval-mode plan --skip-trust` |
| `/gemini-allow` | Write | `gemini --yolo --skip-trust` |

### Skill prefixes

Any skill registered under `extensions/skills/<name>/` is triggered by `/<name>`:

```bash
make cli "/commit-generate"
make cli "/readme-generate private MIT"
```

User message arguments after `/<skill-name>` are passed in as binding context — see [Skill System](Skill-System.md#user-message-is-binding-context).

## REST API

Started by `make app` (default port `:3000`).

| Endpoint | Description |
|---|---|
| `POST /v1/send` | Send a message; body `{sid?, persist?, text}` |
| `POST /v1/key` | Write a value to keychain |
| `GET /v1/key` | Read a value from keychain |
| `GET /v1/tools` | List registered tools |
| `POST /v1/tool/:tool_name` | Invoke a tool directly |
| `GET /v1/session/:sid/status` | Read `status.json` (404 if session missing) |
| `GET /v1/session/:sid/log` | SSE stream of `action.log` (1 s ticker, `: ping` every 15 idle ticks) |

`POST /v1/send` semantics:

| `persist` | `sid` | Result |
|---|---|---|
| `false` (default) | empty | Creates `temp-<uuid>`, reaped after 1 h idle |
| `true` | empty | Creates `http-<uuid>`, retained permanently |
| any | provided | Uses the supplied sid (`persist` is ignored) |

## Environment variables

See [Configuration](Configuration.md) for the full list.

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
