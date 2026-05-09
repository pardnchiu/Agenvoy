# Configuration

> [中文](https://github.com/agenvoy/Agenvoy/wiki/設定檔)

## File layout

```
~/.config/agenvoy/
├── config.json                       Main config (active session, defaults)
├── usage.json                        Token usage tracker
├── runtime.uid                       Server-mode singleton lock
├── mcp.json                          Global MCP servers
├── scheduler/
│   ├── tasks.json                    One-shot scheduled tasks
│   └── crons.json                    Recurring cron tasks
└── sessions/
    └── <sid>/
        ├── bot.md                    Agent persona (frontmatter + body)
        ├── status.json               Active task list / state
        ├── action.log                Tool call audit trail (1 MB rotate, 768 KB target)
        └── mcp.json                  Session-scoped MCP servers
```

History, summaries, and config flags live in ToriiDB under `~/.config/agenvoy/.store/` (managed by ToriiDB itself, not directly user-editable).

## Project configs

```
configs/
├── jsons/
│   ├── providors/                    Provider catalogs (note: spelling is intentional)
│   │   ├── claude.json
│   │   ├── openai.json
│   │   ├── codex.json
│   │   ├── gemini.json
│   │   ├── copilot.json
│   │   └── nvidia.json
│   ├── denied_map.json               Sandbox denied paths
│   ├── exclude_list.json             Listing/walking exclude paths
│   └── white_list.json               Allowed paths
└── prompts/
    ├── system_prompt.md              Main system prompt template
    ├── skill_execution.md            Skill execution discipline
    ├── summary_prompt.md             Summary generation prompt
    ├── summary_merge_prompt.md       Summary merge prompt
    ├── summary_context.md            Summary context injection
    └── discord_system_prompt.md      Discord interface system prompt
```

`compat` provider entries live alongside the static catalogs once added through `agen model add`.

## Environment variables

Loaded from repo-root `.env` via `godotenv` in `cmd/app/main.go init()`.

| Variable | Required | Default | Description |
|---|---|---|---|
| `MAX_HISTORY_MESSAGES` | No | `16` | Max history messages sent per turn |
| `MAX_TOOL_ITERATIONS` | No | `16` | Tool-call iteration cap per request |
| `MAX_SKILL_ITERATIONS` | No | `128` | Tool-call iteration cap during skill execution |
| `MAX_EMPTY_RESPONSES` | No | `8` | Consecutive empty responses tolerated before giving up |
| `MAX_SESSION_TASKS` | No | `3` (cap `10`) | Per-session concurrency limit; excess tasks queue |
| `MAX_SUBAGENT_TIMEOUT_MIN` | No | `10` (cap `60`) | `invoke_subagent` total timeout in minutes |
| `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` | No | `10` (cap `60`) | External CLI subprocess timeout in minutes |
| `OPENAI_API_KEY` | No | — | Enables semantic search via `text-embedding-3-small` |

External CLI agents (`codex` / `gh` / `claude` / `gemini`) are auto-detected via `exec.LookPath`; install the binary on `PATH` to enable, no env flag required.

Numeric variables clamp to the documented cap; values `≤ 0` fall back to default.

## bot.md format

```markdown
---
name: <session display name>     # used by :name routing and invoke_subagent name param
---

<persona content as free-form markdown>
```

The body is rendered into the system prompt's `## Bot Persona` block on every turn. Frontmatter `name` defaults to the session id when not set.

`agen session new <name>` writes both the session directory and a bot.md whose `name` equals `<name>`. `agen session switch <name>` looks up sessions by their bot.md `name` (frontmatter only, no fallback to sid).

## Permission mode

The active permission mode (`single-confirm` vs `always-allow`) is decided by entry point:

| Entry | Mode |
|---|---|
| `agen cli` | `single-confirm` (`AllowAll=false`) |
| `agen run` | `always-allow` (`AllowAll=true`) |
| Discord / REST | `always-allow` |
| Subagent | Inherits parent ctx |

The mode is rendered into the system prompt under `## Permission Mode`. There is no global env var to override it.

## MCP config

Two layers; session overrides global. See [MCP Integration](https://github.com/agenvoy/Agenvoy/wiki/MCP-Integration) for full schema and `${VAR}` expansion behavior.

## Provider config

Provider definitions live under `configs/jsons/providors/` (spelling intentional). Credentials never live in JSON — they live in OS keychain under service `agenvoy`.

## Where things deliberately do **not** live

Some intentional non-locations:

- **Provider API keys** — Never in `config.json`; always in keychain
- **MCP credentials** — Use `${VAR}` placeholders in `mcp.json` and put the actual values in env vars (or keychain via your shell init)
- **Secrets captured by `store_secret`** — Land in keychain only; never in LLM context, history, action.log, or tool args
- **Session history** — In ToriiDB, never in per-session JSON files (this changed in ToriiDB v0.5.0 migration)
- **Tool call results** — Cached in-memory only; not persisted across restarts (except via error_memory and conversation_history)
