# Memory System

The memory layer in Agenvoy has three tiers for conversation memory plus a cross-session error memory tier.

| Tier | Backed by | Scope |
|---|---|---|
| 1. Context window (16 messages + summary) | `history.json` + `summary.json` | session |
| 2. Semantic search (recent) | ToriiDB `DBSessionHist` (vector) | session |
| 3. Full-text archive (all history) | SQLite FTS5 via go-sqlkit | session |
| Error memory | ToriiDB `error_memory` (90d TTL) | cross-session |

## Three-tier conversation memory

### 1. Context window (`max_history_messages`, default 16)

Each session keeps the most recent N messages in full and feeds them into the LLM context window. Anything older remains in `history.json` but is not sent to the LLM.

A rolling summary (`summary.json`) condenses older conversations and is injected into the system prompt at the top of each turn so older context survives beyond the N-message window.

**Incremental cursor:** `summary.meta.json` (per-session) holds `last_message_time` (format `YYYY-MM-DD HH:MM:SS`, extracted from the timestamp in message content). On each `summary.Generate` invocation:

1. `filterAfterTime(histories, cursor)` keeps only `t > cursor` messages
2. Each chunk runs **one** `generatePass` LLM call (the system prompt already includes `{{.Summary}}`=old summary, so merge happens during generation — no separate `mergePass`, avoiding 2x cost)
3. On success, cursor advances to that chunk's max timestamp + `SaveSummary` triggers the mtime gate
4. `generatePass` failure → `return` (don't bill subsequent chunks; next cron tick retries)

### 2. Semantic search — ToriiDB (recent conversations)

The `search_chat_history` tool with `mode=semantic` runs vector similarity search via ToriiDB `db.VSearch`. Each hit triggers a context window expansion: 2 entries before + 1 entry after.

ToriiDB entries are cleaned during `history.json` compaction — entries older than the compact cutoff are removed, keeping ToriiDB focused on recent conversations. Older data lives in SQLite (tier 3).

### 3. Full-text archive — SQLite FTS5 (all history)

Every message written to `history.json` is **dual-written** to SQLite (`~/.config/agenvoy/.store/history.db`) via go-sqlkit. SQLite always holds the complete conversation history, even after `history.json` is compacted.

The `search_chat_history` tool with `mode=keyword` runs FTS5 full-text search on the SQLite archive + ToriiDB substring match on recent entries, combining results.

**Compaction:** when `history.json` exceeds `max_history_bytes` (default 5 MiB), the oldest messages are trimmed to 80% on a complete user+assistant pair boundary. The cutoff timestamp is recorded in SQLite `session_meta.start_at` so that keyword search excludes entries already present in `history.json` (avoiding duplicates). ToriiDB entries older than the cutoff are also removed.

**Backfill:** on first encounter (SQLite has no data for a session but `history.json` has content), the entire existing history is backfilled into SQLite.

**Timestamps:** stored as UTC unix nanoseconds. Timestamps in message content are parsed via `time.ParseInLocation` (local timezone) and converted to UTC for storage. Search queries use `time.Now().UnixNano()` (already UTC).

### Search routing

| `mode` parameter | Source | Use case |
|---|---|---|
| `semantic` (default) | ToriiDB VSearch | "What did we discuss about X?" — meaning-based |
| `keyword` | SQLite FTS5 (archive) + ToriiDB substring (recent) | "Find messages containing 'sandbox'" — exact text |

### Cross-session error memory

Tool failures, resolution paths, and abandoned strategies persist across sessions in `error_memory` with **90-day TTL**. On hit (either via keyword `Contains` or `db.VSearch`), the entry's TTL is refreshed via `db.Expire`.

When the same tool name fails in a future session, `toolCall.go` automatically queries `error_memory` and injects relevant entries as hints into the next assistant turn:

| Outcome on record | Hint behavior |
|---|---|
| `resolved` | Agent must apply the recorded resolution |
| `failed` / `abandoned` | Agent must avoid the recorded strategy |

## Storage layout

| Store | Content | Lifecycle |
|---|---|---|
| `history.json` | Recent messages (hot, LLM reads every turn) | Auto-compacted at 5 MiB |
| ToriiDB `DBSessionHist` | Recent messages with embeddings | Cleaned on compact (entries < cutoff removed) |
| SQLite `messages` | All messages ever written (dual-write) | Cleared on reset / remove-session |
| SQLite `session_meta` | `start_at` — compact cutoff timestamp | Cleared on reset / remove-session |
| `summary.json` | Rolling summary blob | Survives reset |
| ToriiDB `error_memory` | Tool error records with resolution metadata | 90d TTL (refresh on hit) |

## Reset / remove behavior

| Operation | `history.json` | ToriiDB `DBSessionHist` | SQLite (messages + meta) | `summary.json` |
|---|---|---|---|---|
| Compact (auto) | Trimmed to 80% | Entries < cutoff removed | Untouched (already has all data) | Untouched |
| Reset (`/reset`) | Deleted | Cleared | Cleared | Preserved |
| Remove session | Directory deleted | Cleared | Cleared | Directory deleted |

## Migration note

Sessions and error memory used to live under per-session JSON files. Since ToriiDB v0.5.0 they are inside the embedded store. Do not reintroduce JSON paths.

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
