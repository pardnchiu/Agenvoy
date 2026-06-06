# KuraDB RAG

> [中文](KuraDB-RAG.zh.md)

KuraDB is the in-process RAG (Retrieval-Augmented Generation) provider for Agenvoy. It runs as a daemon-managed child process and exposes two search tools (`list_rag`, `search_rag`) to the agent.

## What it is

KuraDB ([pardnchiu/KuraDB](https://github.com/pardnchiu/KuraDB)) is a self-developed local document index that:

- Indexes user files (notes, inbox, code, …) into multiple named databases
- Provides keyword search via `gse` tokenization (Chinese-aware)
- Provides semantic search via OpenAI embeddings (`text-embedding-3-small`)
- Runs entirely on the user's machine — no external service

Agenvoy talks to KuraDB over a local HTTP API (random port written to `~/.config/kuradb/endpoint` at startup).

## Lifecycle

KuraDB lives in `internal/runtime/kuradb/`:

| File | Responsibility |
|---|---|
| `kuradb.go` | Public surface: `BinaryPath`, `EndpointExists()`, `ReadEndpoint()`, `BinaryInstalled()`, `HasOpenAIKey()`, `SetOpenAIKey()` |
| `run.go` | `RunChild(ctx)` — `exec.Cmd` start + `StdoutPipe`/`StderrPipe` → slog; 5-second crash backoff; health check goroutine polls `<endpoint>/api/health` every minute (5s timeout), 3 consecutive failures → auto-disable |

### Daemon orchestration (`cmd/app/cmdDeamon.go::reloadKuradb`)

The daemon controls KuraDB via fsnotify on `~/.config/agenvoy/config.json`:

1. On config change with `kuradb_enabled=true`, three gates check **before** spawning:
   - `kuradb.BinaryInstalled()` — `/usr/local/bin/kura` must exist
   - `kuradb.HasOpenAIKey()` — `OPENAI_API_KEY` must be in keychain (`agenvoy` service)
2. Any gate failure → **silent return** (the user explicitly chose this behavior — "直接忽視比較實在": don't log, don't auto-disable, don't write config)
3. Pass → spawn child via `RunChild`; KuraDB writes endpoint URL to `~/.config/kuradb/endpoint`
4. Healthcheck goroutine starts; 3 consecutive failures → write `kuradb_enabled=false` + remove endpoint file + **explicit** `reloadKuradb()` call (not via fsnotify async, to avoid 200ms race window)

### Crash recovery

`RunChild` wraps the child in a 5-second backoff loop. Stdout/stderr are piped through `bufio.Scanner` into `slog` so KuraDB errors land in `daemon.log` instead of being dropped.

## Tool registration

The two RAG tools live in `internal/runtime/kuradb/tool/` and register at all three entry points (`cmd/app/{main,cmdDeamon,newTUI}.go`) via explicit `kuradbTool.Register()` calls (not `init()` — `init()` fires before `filesystem.Init()`, gate check would always fail).

| Tool | Description |
|---|---|
| `list_rag` | List available KuraDB databases (e.g. `notes`, `inbox`, `code`) |
| `search_rag` | Search a database by keyword (`mode=keyword`, `gse` tokenization) or semantic (`mode=semantic`, OpenAI embeddings) |

Tool gate is single-condition `cfg.KuradbEnabled` — the per-handler `ReadEndpoint()` call is the second-line defense if the endpoint disappears mid-turn.

## Per-turn dynamic exclusion

`exec.Execute()` checks `kuradb.EndpointExists()` after `NewExecutor`. When false, the two RAG tools are appended to `data.ExcludeTools`, and the existing filter mechanism strips them from `exec.Tools` for that turn.

The result: the LLM **never sees** `list_rag` / `search_rag` tools when the endpoint is down — not even the stub names. The conditional "when `list_rag` / `search_rag` tools are present" guidance in the system prompt then naturally inactivates.

**Why this matters:** without dynamic exclusion, the LLM would see RAG tool stubs at startup race (before KuraDB child spawns), call them, and get errors — confusing both LLM and user.

## `/kuradb` TUI wizard

Enable / disable is exposed only through the TUI (no CLI subcommand by design — install.sh + sudo prompts need a real TTY):

```
/kuradb         → popup: enable | disable
```

### Enable flow

1. Wizard checks `HasOpenAIKey()`; if missing, opens a `popupText` to collect the key → `keychain.Set("OPENAI_API_KEY", value)` (service: `agenvoy`)
2. `tea.ExecProcess` runs the install script:
   ```
   curl -fsSL https://cloud.agenvoy.com/KuraDB/install.sh | bash
   ```
   The TTY is handed to the child so `sudo` prompts and package manager output work
3. Verifies `kura` binary at `/usr/local/bin/kura`; writes `kuradb_enabled=true` to config.json
4. Daemon picks up via fsnotify → `reloadKuradb()` spawns the child → endpoint file appears → tools become callable

### Disable flow

1. `tea.ExecProcess` runs `sudo rm /usr/local/bin/kura`
2. Writes `kuradb_enabled=false` to config.json
3. Daemon `reloadKuradb()` signals the running child to shut down

## RAG-first prompting

When `list_rag` / `search_rag` tools are loaded, the base system prompt requires that **the first wave of tool calls for any information query** be `list_rag` + `search_rag` — external web/search tools are **secondary** (used to fill gaps), not fallback or substitute.

This is enforced in `configs/prompts/system_prompt.md`; the rule self-deactivates when KuraDB is off (because `list_rag` / `search_rag` won't be in the tool list).

## Files & paths

| Path | Purpose |
|---|---|
| `/usr/local/bin/kura` | KuraDB binary (installed by `install.sh`) |
| `~/.config/kuradb/endpoint` | Plaintext URL, written by KuraDB on startup, removed on disable |
| `~/.config/kuradb/` | KuraDB-side config / data dir (managed by KuraDB itself) |
| Keychain `agenvoy/OPENAI_API_KEY` | Shared with Agenvoy's other OpenAI-using features |

## Related pages

- [Tools](Tools.md#rag) — `list_rag` / `search_rag` tool definitions
- [Memory System](Memory-System.md) — how KuraDB complements ToriiDB-backed conversation memory
- [CLI Reference](CLI-Reference.md) — `/kuradb` TUI command
- [Configuration](Configuration.md#kuradb) — config keys and paths

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
