# 記憶系統

> [English](Memory-System.md)

Agenvoy 記憶層有四層 ToriiDB-backed 的對話記憶，外加第五層選用——KuraDB——做外部文件 RAG。

| 層 | 後端 | 範圍 |
|---|---|---|
| 當前 context（16-msg 視窗） | in-memory + `DBSessionHist` | session |
| Rolling summary | `DBSessionSummary` | session |
| 對話歷史搜尋 | `DBSessionHist`（keyword + vector） | session |
| Error memory | `error_memory`（90d TTL） | 跨 session |
| 外部文件 RAG | [KuraDB](KuraDB-RAG.zh.md)（in-process child） | 跨 session、使用者自管的具名資料庫 |

## 四層架構（對話記憶，ToriiDB）

### 1. 當前上下文（`MAX_HISTORY_MESSAGES`，預設 16）

每個 session 保留最近 N 則完整訊息直接餵進 LLM context window。超過的內容自動壓縮進 rolling summary。

`MAX_HISTORY_MESSAGES` 是上限；數值越小 token 越省但遺忘越快。

### 2. Rolling Summary

當最近 N 則的視窗滑動，被擠出去的訊息會經由 `summary_prompt.md` 摘要後寫入 `DBSessionSummary`。Summary 在每輪 system prompt 頂部注入，確保舊脈絡不丟。

**增量游標：** `summary.meta.json`（per-session）存 `last_message_time`（格式 `YYYY-MM-DD HH:MM:SS`，從 message content 開頭的 `當前時間: ...` 抓）。每次 `summary.Generate` 呼叫：

1. `filterAfterTime(histories, cursor)` 只保留 `t > cursor` 的 messages
2. 每 chunk **一次** `generatePass` LLM 呼叫（system prompt 內已塞 `{{.Summary}}`=old summary，merge 在生成時完成 —— 無獨立 `mergePass`，避 2× cost）
3. 成功後 cursor 推進到該 chunk 內最大 timestamp + `SaveSummary` 觸發 mtime gate
4. `generatePass` 失敗即 `return`（不雙計費已成功 chunk，下輪 cron retry）

**為何用 timestamp 而非 count：** `summary.json` 欄位（`discussion_log` / `key_data`）有上限會裁剪，count 對應的「summary 覆蓋範圍」不穩。Message timestamp 是 message 自帶硬資料，不受裁剪影響。

**Migration：** cursor 為空但 `summaryMap` 非空（首次升級）時 → `SaveSummaryMeta(latestMessageTime(histories))` + `SaveSummary` 寫 gate，當輪跳過 —— 避免首次升級後重跑全部歷史造成 token 爆量。

> Summary 自動剝除已於 commit `a33cbef` 移除 —— 摘要**只**在明確觸發時產生。**勿再加回自動剝除**。

### 3. 語意對話歷史搜尋

`search_conversation_history` 工具 **keyword + semantic 並聯執行**：

- **Keyword 路徑** —— 對歷史 `Contains` scan，可選 `time_range` 預過濾（1d / 7d / 1m / 1y）
- **Semantic 路徑** —— `db.VSearch(ctx, keyword, sid+":*", k)` 餘弦 top-K；`time_range` **不**套用（語意相關性才是訊號）

`limit ∈ [8, 16, 32]` 是 per-source cap。兩路徑各撈 `limit/2`，結果聯集去重。

每筆命中觸發 context window 擴展：前 2 後 1（不對稱 —— 前文看 setup、後文看 resolution）。相鄰 index 形成連續片段；不相鄰用空行分隔。

缺 `OPENAI_API_KEY` 時 semantic 路徑靜默回空，只走 keyword scan。

### 4. 跨 Session Error Memory

工具失敗、解法、放棄策略跨 session 持久化在 `error_memory`，**90 天 TTL**。命中（無論 keyword `Contains` 或 `db.VSearch`）即透過 `db.Expire` 續期。

未來在另一 session 同個 tool 又失敗時，`toolCall.go` 自動查 `error_memory` 並把相關紀錄當 hint 注入下輪 assistant turn：

| 紀錄 outcome | Hint 行為 |
|---|---|
| `resolved` | Agent 必須套用記錄的解法 |
| `failed` / `abandoned` | Agent 必須避開記錄的策略 |

## 儲存配置

| Database | 內容 | TTL |
|---|---|---|
| `DBSessionHist` | 每筆 user／assistant turn 一個 entry，經 `text-embedding-3-small` 向量化 | 無（除非 session 是 `temp-*` 被 reap） |
| `DBSessionSummary` | 每 session 一個 summary blob | 無 |
| `DBConfig` | session 層 config 旗標 | 無 |
| `error_memory` | 工具錯誤紀錄含 resolution metadata | 90d（命中即續期） |

## 為什麼用 ToriiDB

- **Embedded** —— 不需外部服務
- **Vector search** —— `SetVector` + `VSearch` cosine 相似度
- **TTL with refresh** —— `Expire` 讓常被引用的 entry 保熱
- **Lazy value getter** —— `Entry.Value` 是 `func() string`，scan 時不必 materialize 全部結果

## 外部文件 RAG（KuraDB）

上述四層皆服務對話記憶——歷史對話、摘要、錯誤紀錄。要查詢**使用者自管的文件集**（筆記、收件匣、程式碼倉……），Agenvoy 委派給 [KuraDB](KuraDB-RAG.zh.md)——daemon 在 `kuradb_enabled=true` 時 spawn 的 in-process child。

KuraDB 對 agent 暴露三個工具（`rag_list_db` / `rag_search_keyword` / `rag_search_semantic`），endpoint 不存在時 per-turn 動態排除。載入時 system prompt 強制 information query 第一波先呼這三者（外部 web 工具退為補足角色）。

這個切割是刻意的：ToriiDB 是整合在 runtime 的記憶層（不能停用）；KuraDB 是 opt-in 的索引知識庫（透過 `/kuradb` TUI 啟用）。

## 遷移備註

Session 與 error memory 過去存在 per-session JSON 檔。從 ToriiDB v0.5.0 起改進 embedded store。**勿恢復 JSON 路徑**。
