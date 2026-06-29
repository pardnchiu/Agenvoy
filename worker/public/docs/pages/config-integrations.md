# Integration Config

## MCP config

Two layers; session overrides global. See MCP Integration for full schema and `${VAR}` expansion behavior.

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

Enabled state is `kuradb_enabled: bool` in config.json. Toggle via `/feature kuradb` in the TUI (no CLI subcommand — install.sh + sudo need a real TTY). See KuraDB RAG for full lifecycle.

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
