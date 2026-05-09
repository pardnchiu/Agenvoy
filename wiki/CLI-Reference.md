# CLI Reference

> [中文](https://github.com/agenvoy/Agenvoy/wiki/命令列參考)

## Top-level dispatch

`agen` parses `os.Args[1]` and routes to one of six sub-groups; running with no subcommand starts the full app (`runApp`).

```bash
agen                                           # TUI + Discord + REST stack
agen model   {add|remove|list|planner|reasoning}
agen skill   {list}
agen session {new|switch|config} [name]
agen mcp     {list|add|remove}
agen cli     <input...>                        # one-shot, requires per-tool confirm
agen run     <input...>                        # one-shot, auto-approves all tools
```

### `agen model`

| Subcommand | Action |
|---|---|
| `add` | Interactive provider/model add (writes credentials to keychain) |
| `remove` (alias `rm`) | Interactive provider/model remove |
| `list` | List registered models |
| `planner` | Choose the planner model |
| `reasoning` | Set planner reasoning effort: `low` / `medium` / `high` / `xhigh` |

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

### `agen skill`

`agen skill` (no subcommand) and `agen skill list` both list available skills under `extensions/skills/`.

## `make` shortcuts

```bash
make build                      # Compile and install to /usr/local/bin/agen
make app                        # Full stack (TUI + Discord + REST API)
make discord                    # Legacy Discord-only server
make cli <input...>             # agen cli <input...>
make run <input...>             # agen run <input...>
make model   [add|remove|list|planner|reasoning]
make skill   [list]
make session [new|switch|config] [name]
make mcp     [list|add|remove]
make test                       # go test ./test/... -v -timeout 60s
```

## TUI shortcuts

Main view (`Content` / `Logs`):

| Key | Action |
|---|---|
| `i` | Open Message input (`> ` prompt, multi-line; `Shift+Enter` inserts newline on capable terminals) |
| `c` | Open Command input (`$ ` prompt, single-line) |
| `Enter` | Submit (Message: send + clear; Command: run) |
| `Esc` | Close input / popup |
| `Tab` | Toggle Content / Logs view; in input pages toggles Command ↔ Message |
| `Ctrl+P` | Toggle co-work dashboard (Sessions / Log / Pending three-panel) |
| `h` / `l` / arrows | Navigate panels |
| `j` / `k` | Scroll active view |
| `Ctrl+C` | Cancel current execution |

Co-work dashboard:

- **Sessions** — left pane, lists tracked `cli-*` and `http-*` sessions (no `temp-*` / `dc-*`)
- **Log** — center, tails the selected session's `action.log` formatted CLI-style
- **Pending** — right pane, only visible when `pending.Snapshot()` returns ≥1 entry; selecting an item jumps Sessions to that sid

## Input prefixes

Resolution order in `exec.Run()` (CLI / TUI only — Discord and HTTP do not parse `:name`):

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

User message arguments after `/<skill-name>` are passed in as binding context — see [Skill System](https://github.com/agenvoy/Agenvoy/wiki/Skill-System#user-message-is-binding-context).

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

See [Configuration](https://github.com/agenvoy/Agenvoy/wiki/Configuration) for the full list.
