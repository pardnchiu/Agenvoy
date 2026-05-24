# Memory System

> [中文](Memory-System.zh.md)

The memory layer in Agenvoy has four ToriiDB-backed tiers for conversation memory, plus a fifth optional tier — KuraDB — for external document RAG.

| Tier | Backed by | Scope |
|---|---|---|
| Current context (16-msg window) | in-memory + `DBSessionHist` | session |
| Rolling summary | `DBSessionSummary` | session |
| Conversation history search | `DBSessionHist` (keyword + vector) | session |
| Error memory | `error_memory` (90d TTL) | cross-session |
| External document RAG | [KuraDB](KuraDB-RAG.md) (in-process child) | cross-session, user-curated databases |

## Four-tier architecture (conversation memory, ToriiDB)

### 1. Current context (`MAX_HISTORY_MESSAGES`, default 16)

Each session keeps the most recent N messages in full and feeds them into the LLM context window. Anything older is compressed into the rolling summary.

`MAX_HISTORY_MESSAGES` is the cap; lower values reduce token cost but increase forgetfulness.

### 2. Rolling summary

When the recent-N window slides, the displaced messages are summarized via `summary_prompt.md` and persisted in `DBSessionSummary`. The summary is injected into the system prompt at the top of each turn so older context survives.

**Incremental cursor:** `summary.meta.json` (per-session) holds `last_message_time` (format `YYYY-MM-DD HH:MM:SS`, extracted from message content's opening `當前時間: ...`). On each `summary.Generate` invocation:

1. `filterAfterTime(histories, cursor)` keeps only `t > cursor` messages
2. Each chunk runs **one** `generatePass` LLM call (the system prompt already includes `{{.Summary}}`=old summary, so merge happens during generation — no separate `mergePass`, avoiding 2× cost)
3. On success, cursor advances to that chunk's max timestamp + `SaveSummary` triggers the mtime gate
4. `generatePass` failure → `return` (don't bill subsequent chunks; next cron tick retries)

**Why timestamp not count:** `summary.json` fields (`discussion_log` / `key_data`) get pruned, so message-count-based coverage drifts. Message timestamps are immutable, unbroken signal.

**Migration:** if cursor is empty but `summaryMap` is non-empty (first upgrade), `SaveSummaryMeta(latestMessageTime(histories))` + `SaveSummary` writes the gate and the current tick skips — avoiding token blow-up from re-summarizing N messages on first upgrade.

> Summary auto-stripping was removed in commit `a33cbef` — summaries are produced **only** on explicit trigger. Do not re-introduce automatic stripping.

### 3. Semantic conversation history search

The `search_conversation_history` tool runs **keyword + semantic in parallel**:

- **Keyword path** — `Contains` scan over history with optional `time_range` pre-filter (1d / 7d / 1m / 1y)
- **Semantic path** — `db.VSearch(ctx, keyword, sid+":*", k)` cosine top-K; `time_range` does **not** apply (semantic relevance is the signal)

`limit ∈ [8, 16, 32]` is a per-source cap. Both paths fetch `limit/2`, then results merge and dedupe by key.

Each hit triggers a context window expansion: 2 entries before + 1 entry after (asymmetric — the prior reveals setup, the following reveals resolution). Adjacent indices form contiguous blocks; gaps are separated by blank lines.

When `OPENAI_API_KEY` is missing, the semantic path silently returns empty and only keyword scan applies.

### 4. Cross-session error memory

Tool failures, resolution paths, and abandoned strategies persist across sessions in `error_memory` with **90-day TTL**. On hit (either via keyword `Contains` or `db.VSearch`), the entry's TTL is refreshed via `db.Expire`.

When the same tool name fails in a future session, `toolCall.go` automatically queries `error_memory` and injects relevant entries as hints into the next assistant turn:

| Outcome on record | Hint behavior |
|---|---|
| `resolved` | Agent must apply the recorded resolution |
| `failed` / `abandoned` | Agent must avoid the recorded strategy |

## Storage layout

| Database | Content | TTL |
|---|---|---|
| `DBSessionHist` | One entry per user/assistant turn, vectorized via `text-embedding-3-small` | None (unless session is `temp-*` and reaped) |
| `DBSessionSummary` | One summary blob per session | None |
| `DBConfig` | Session-level config flags | None |
| `error_memory` | Tool error records with resolution metadata | 90 d (refresh on hit) |

## Why ToriiDB

- **Embedded** — no external service required
- **Vector search** — `SetVector` + `VSearch` with cosine similarity
- **TTL with refresh** — `Expire` keeps frequently relevant entries hot
- **Lazy value getter** — `Entry.Value` is `func() string` to avoid materializing every result during scans

## External document RAG (KuraDB)

The four tiers above all serve conversation memory — past chats, summaries, error records. For querying **user-curated document collections** (notes, inbox, code repos, …), Agenvoy delegates to [KuraDB](KuraDB-RAG.md), an in-process child process spawned by the daemon when `kuradb_enabled=true`.

KuraDB exposes three tools to the agent (`rag_list_db` / `rag_search_keyword` / `rag_search_semantic`), per-turn dynamically excluded when the endpoint file is missing. When loaded, the system prompt forces them to fire **first** for any information query (external web tools become gap-filling secondary).

This split is deliberate: ToriiDB is integrated runtime memory (you can't disable it); KuraDB is an opt-in indexed knowledge base (enable via `/kuradb` TUI command).

## Migration note

Sessions and error memory used to live under per-session JSON files. Since ToriiDB v0.5.0 they are inside the embedded store. Do not reintroduce JSON paths.
