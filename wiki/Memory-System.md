# Memory System

> [中文](https://github.com/agenvoy/Agenvoy/wiki/記憶系統)

The memory layer in Agenvoy has four tiers, all backed by [ToriiDB](https://github.com/pardnchiu/ToriiDB) (embedded KV + vector search).

## Four-tier architecture

### 1. Current context (`MAX_HISTORY_MESSAGES`, default 16)

Each session keeps the most recent N messages in full and feeds them into the LLM context window. Anything older is compressed into the rolling summary.

`MAX_HISTORY_MESSAGES` is the cap; lower values reduce token cost but increase forgetfulness.

### 2. Rolling summary

When the recent-N window slides, the displaced messages are summarized via `summary_prompt.md` + `summary_merge_prompt.md` and persisted in `DBSessionSummary`. The summary is injected into the system prompt at the top of each turn so older context survives.

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

## Migration note

Sessions and error memory used to live under per-session JSON files. Since ToriiDB v0.5.0 they are inside the embedded store. Do not reintroduce JSON paths.
