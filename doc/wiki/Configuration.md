# Configuration

> [中文](Configuration.zh.md)

## File layout

```
~/.config/agenvoy/
├── config.json                       Main config (active session, dispatcher_model, kuradb_enabled, t_enabled, d_enabled, compats[])
├── usage.json                        Token usage tracker
├── runtime.uid                       Server-mode singleton lock (daemon-only writer)
├── mcp.json                          Global MCP servers
├── allow_skill                       Global skill always-allow list (one name per line)
├── .telegram                         Authorized Telegram chat IDs (one per line, written after OTP success)
├── scheduler/
│   ├── tasks.json                    One-shot scheduled tasks
│   └── crons.json                    Recurring cron tasks
├── download/                         Inbound chat attachments + outbound generated images (agenvoy-img-<uuid>.png)
├── skills/scheduler/                 Isolated scheduler skill dirs (<short>-<hash8>/SKILL.md)
└── sessions/
    └── <sid>/
        ├── bot.md                    Agent persona (frontmatter + body)
        ├── status.json               Active task list / state
        ├── action.log                Tool call audit trail (1 MB rotate, 768 KB target; foreign-process lines prefixed)
        ├── summary.meta.json         {last_message_time: YYYY-MM-DD HH:MM:SS} — incremental summary cursor
        ├── input_history             Per-session TUI input history
        └── mcp.json                  Session-scoped MCP servers

~/.config/kuradb/
└── endpoint                          Plaintext URL (random port), written by KuraDB on spawn, removed on disable

<project-root>/.agenvoy/
└── allow_skill                       Project-scoped skill always-allow list (union with global at load time)
```

History, summaries, error memory and config flags live in ToriiDB under `~/.config/agenvoy/.store/` (managed by ToriiDB itself, not directly user-editable).

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
    ├── discord_system_prompt.md      Discord interface system prompt
    └── telegram_system_prompt.md     Telegram interface system prompt
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
| `AGENT_SEND_TIMEOUT_SECONDS` | No | `600` | Exec-layer ceiling on `Agent.Send`; wraps `context.WithTimeout` around the provider call. Mainly relevant for codex SSE (10m client timeout); for non-SSE providers, `Client.Timeout=5m` fires first |
| `OPENAI_API_KEY` | No | — | Enables semantic search via `text-embedding-3-small` and KuraDB embedding |

External CLI agents (`codex` / `gh` / `claude` / `gemini`) are auto-detected via `exec.LookPath`; install the binary on `PATH` to enable, no env flag required.

Numeric variables clamp to the documented cap; values `≤ 0` fall back to default.

## bot.md format

```markdown
***
name: <session display name>     # used by :name routing and invoke_subagent name param
***

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
| Telegram | `single-confirm` (`AllowAll=false`; confirm gate uses Telegram inline-keyboard SendSelect) |
| Subagent | Inherits parent ctx |

The mode is rendered into the system prompt under `## Permission Mode`. There is no global env var to override it.

## MCP config

Two layers; session overrides global. See [MCP Integration](MCP-Integration.md) for full schema and `${VAR}` expansion behavior.

## Provider config

Provider definitions live under `configs/jsons/providors/` (spelling intentional). Credentials never live in JSON — they live in OS keychain under service `agenvoy`.

### Compat provider URL storage split

`compat` provider URLs use a **two-storage** model:

| What | Where | Why |
|---|---|---|
| URL (e.g. `http://host:8000/v1`) | `~/.config/agenvoy/config.json` `compats[].URL` | Non-secret, user-editable |
| API key (`COMPAT_<NAME>_API_KEY`) | OS keychain | Secret |

URL convention follows Zed: the user enters the URL up to `/v1` (e.g. `http://localhost:11434/v1`), and `compat/send.go` appends only `/chat/completions`. `compat.New` reads URL via `session.GetCompatURL(instanceName)` — **not** keychain. There is no `COMPAT_<NAME>_URL` keychain key (intentionally removed: a historical bug had the TUI writing to config while runtime read keychain, always falling back to localhost).

## KuraDB

Enabled state is `kuradb_enabled: bool` in config.json. Toggle via `/feature kuradb` in the TUI (no CLI subcommand — install.sh + sudo need a real TTY). See [KuraDB RAG](KuraDB-RAG.md) for full lifecycle.

| Key | Location |
|---|---|
| `kuradb_enabled` | `config.json` |
| `OPENAI_API_KEY` | keychain (`agenvoy` service) — shared with semantic search |
| Endpoint URL (runtime) | `~/.config/kuradb/endpoint` (plaintext, random port per spawn) |
| Binary | `/usr/local/bin/kura` (hardcoded in install.sh) |

## Telegram / Discord enablement

| Key | Location |
|---|---|
| `telegram_enabled` / `discord_enabled` | `config.json` |
| `TELEGRAM_TOKEN` / `DISCORD_TOKEN` | keychain (`agenvoy` service) |
| Authorized chat IDs | `~/.config/agenvoy/.telegram` (one chat ID per line, written after 6-digit OTP verification succeeds) |
| Authorized Discord channels | Set via guild mention + per-server `d_allowed` config |

## Where things deliberately do **not** live

Some intentional non-locations:

- **Provider API keys** — Never in `config.json`; always in keychain
- **MCP credentials** — Use `${VAR}` placeholders in `mcp.json` and put the actual values in env vars (or keychain via your shell init)
- **Secrets captured by `store_secret`** — Land in keychain only; never in LLM context, history, action.log, or tool args
- **Session history** — In ToriiDB, never in per-session JSON files (this changed in ToriiDB v0.5.0 migration)
- **Tool call results** — Cached in-memory only; not persisted across restarts (except via error_memory and conversation_history)

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
