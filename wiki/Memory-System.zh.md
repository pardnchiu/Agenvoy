# 記憶系統

> [English](https://github.com/agenvoy/Agenvoy/wiki/Memory-System)

Agenvoy 記憶層分四層，底層皆走 [ToriiDB](https://github.com/pardnchiu/ToriiDB)（embedded KV + 向量搜尋）。

## 四層架構

### 1. 當前上下文（`MAX_HISTORY_MESSAGES`，預設 16）

每個 session 保留最近 N 則完整訊息直接餵進 LLM context window。超過的內容自動壓縮進 rolling summary。

`MAX_HISTORY_MESSAGES` 是上限；數值越小 token 越省但遺忘越快。

### 2. Rolling Summary

當最近 N 則的視窗滑動，被擠出去的訊息會經由 `summary_prompt.md` + `summary_merge_prompt.md` 摘要後寫入 `DBSessionSummary`。Summary 在每輪 system prompt 頂部注入，確保舊脈絡不丟。

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

## 遷移備註

Session 與 error memory 過去存在 per-session JSON 檔。從 ToriiDB v0.5.0 起改進 embedded store。**勿恢復 JSON 路徑**。
