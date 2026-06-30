# Sessions & Agents

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
| `temp-*` | Reaped after 30 min idle (default for `POST /v1/send` and subagent sessions) |

Cleanup runs every 30 minutes via cron (and once on startup) against the `temp-*` prefix only — `cli-*`, `http-*`, `dc-*`, and `tg-*` are never auto-reaped.

## bot.md — Agent Persona

Each session can declare its own persona:

```markdown
***
name: mobile-builder
***

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
