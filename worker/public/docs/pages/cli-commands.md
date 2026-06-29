# CLI Commands

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
| `add` | Interactive add — name, transport (Local stdio / Remote HTTP), fields, scope (Global / pick a session) |
| `remove` | Interactive remove with scope label |

### `agen stop`

SIGTERM the running daemon (5 s grace period, then SIGKILL); clears `~/.config/agenvoy/runtime.uid`. Prints `No daemon running.` and exits 0 if no daemon is alive.

### `agen update`

Always-overwrite update to the latest release. Downloads `https://agenvoy.com/scripts/update.sh` to a `/tmp/agenvoy-update-*.sh` file, executes it via `bash`, and removes the temp file on completion (SIGINT/SIGTERM also cleaned). The script clones the latest tag into `mktemp -d "${TMPDIR:-/tmp}/agenvoy-update.XXXXXX"`, runs `make build`, and prints a summary box pointing to `agen` for the next launch. Daemon keeps the old inode after replacement — run `agen stop` and re-attach to pick up the new build.

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

## Input prefixes

Resolution order in `exec.Run()` (CLI / TUI / Telegram only — Discord and HTTP do not parse `:name`):

1. **`:name`** — session override (one-shot routing without changing primary pointer)
2. **`MatchExternal`** — external CLI agent dispatch (`/claude`, `/codex`, etc.)
3. **`MatchSkillCall`** — skill activation (`/<skill-name>`)

### `:name` session override

```bash
make cli ":ship-v0.20 /commit-generate"
```

Composable with skills and external agents — order resolves left to right (`:bot` then external then skill then execute).

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

User message arguments after `/<skill-name>` are passed in as binding context.

## Auto mode

Press **Shift+Tab** to toggle auto mode. The current mode is shown at the bottom-left of the TUI:

- `[safe]` (default) — tool calls require user confirmation before execution
- `[auto]` — all tool calls are automatically approved (`allowAll = true`); sandbox and validator still apply

Auto mode is session-local and resets when the TUI restarts. It can also be set at launch via `agen --allow-all`.

## Environment variables

See the Configuration page for the full list.
