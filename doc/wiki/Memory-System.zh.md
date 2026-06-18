# 記憶系統

> [English](Memory-System.md)

Agenvoy 記憶層有三層對話記憶、一層跨 session 錯誤記憶，外加選用的 KuraDB 做外部文件 RAG。

| 層 | 後端 | 範圍 |
|---|---|---|
| 1. 上下文視窗（16 筆 + summary） | `history.json` + `summary.json` | session |
| 2. 語意搜尋（近期） | ToriiDB `DBSessionHist`（向量） | session |
| 3. 全文歸檔（全歷史） | SQLite FTS5 via [go-sqlite](https://github.com/pardnchiu/go-sqlite) | session |
| Error memory | ToriiDB `error_memory`（90d TTL） | 跨 session |
| 外部文件 RAG | [KuraDB](KuraDB-RAG.zh.md)（in-process child） | 跨 session、使用者自管的具名資料庫 |

## 三層對話記憶

### 1. 上下文視窗（`max_history_messages`，預設 16）

每個 session 保留最近 N 則完整訊息直接餵進 LLM context window。超過的部分仍在 `history.json`，但不送進 LLM。

Rolling summary（`summary.json`）濃縮舊對話，每輪注入 system prompt 頂部，確保 N 筆視窗以外的脈絡不丟。

**增量游標：** `summary.meta.json`（per-session）存 `last_message_time`（格式 `YYYY-MM-DD HH:MM:SS`，從 message content 開頭的 `當前時間: ...` 抓）。每次 `summary.Generate` 呼叫：

1. `filterAfterTime(histories, cursor)` 只保留 `t > cursor` 的 messages
2. 每 chunk **一次** `generatePass` LLM 呼叫（system prompt 內已塞 `{{.Summary}}`=old summary，merge 在生成時完成 —— 無獨立 `mergePass`，避 2× cost）
3. 成功後 cursor 推進到該 chunk 內最大 timestamp + `SaveSummary` 觸發 mtime gate
4. `generatePass` 失敗即 `return`（不雙計費已成功 chunk，下輪 cron retry）

### 2. 語意搜尋 — ToriiDB（近期對話）

`search_chat_history` 工具以 `mode=semantic` 走 ToriiDB `db.VSearch` 向量相似搜尋。每筆命中觸發 context window 擴展：前 2 後 1。

ToriiDB entries 在 `history.json` compact 時清理——早於裁剪點的 entries 會被移除，讓 ToriiDB 聚焦近期對話。更舊的資料由 SQLite（第 3 層）承接。

### 3. 全文歸檔 — SQLite FTS5（全歷史）

每筆寫入 `history.json` 的訊息會**同步雙寫**到 SQLite（`~/.config/agenvoy/.store/history.db`），透過 [pardnchiu/go-sqlite](https://github.com/pardnchiu/go-sqlite)。SQLite 永遠持有完整對話歷史，即使 `history.json` 被裁剪。

`search_chat_history` 工具以 `mode=keyword` 走 SQLite FTS5 全文搜尋（歸檔）+ ToriiDB 子字串比對（近期），合併結果輸出。

**裁剪（Compact）：** 當 `history.json` 超過 `max_history_bytes`（預設 5 MiB），從最舊端裁剪至 80%（必在完整 user+assistant pair 邊界）。裁剪點 timestamp 記錄到 SQLite `session_meta.start_at`，讓 keyword 搜尋排除 `history.json` 已有的內容（避免重複）。同時清除 ToriiDB 中早於裁剪點的 entries。

**首次回填：** 首次遇到（SQLite 無該 session 資料但 `history.json` 有既存內容）時，整段既存歷史會回填進 SQLite。

**時間戳：** 以 UTC unix nanoseconds 儲存。message content 中的 `當前時間:` 用 `time.ParseInLocation`（本地時區）解析後轉 UTC 存入。搜尋用 `time.Now().UnixNano()`（本身即 UTC）。

### 搜尋路由

| `mode` 參數 | 來源 | 使用場景 |
|---|---|---|
| `semantic`（預設） | ToriiDB VSearch | 「我們之前討論 X 時怎麼決定的？」—— 依語意 |
| `keyword` | SQLite FTS5（歸檔）+ ToriiDB 子字串（近期） | 「找包含 'sandbox' 的訊息」—— 精確文字 |

### 跨 Session Error Memory

工具失敗、解法、放棄策略跨 session 持久化在 `error_memory`，**90 天 TTL**。命中即透過 `db.Expire` 續期。

未來在另一 session 同個 tool 又失敗時，`toolCall.go` 自動查 `error_memory` 並把相關紀錄當 hint 注入下輪 assistant turn：

| 紀錄 outcome | Hint 行為 |
|---|---|
| `resolved` | Agent 必須套用記錄的解法 |
| `failed` / `abandoned` | Agent 必須避開記錄的策略 |

## 儲存配置

| 儲存 | 內容 | 生命週期 |
|---|---|---|
| `history.json` | 近期訊息（熱，LLM 每輪直讀） | 5 MiB auto-compact |
| ToriiDB `DBSessionHist` | 近期訊息含 embeddings | compact 時清理（< 裁剪點移除） |
| SQLite `messages` | 所有曾寫入的訊息（雙寫） | reset／remove-session 時清除 |
| SQLite `session_meta` | `start_at` — compact 裁剪點 timestamp | reset／remove-session 時清除 |
| `summary.json` | Rolling summary blob | 跨 reset 存活 |
| ToriiDB `error_memory` | 工具錯誤紀錄含 resolution metadata | 90d TTL（命中即續期） |

## Reset / Remove 行為

| 操作 | `history.json` | ToriiDB `DBSessionHist` | SQLite（messages + meta） | `summary.json` |
|---|---|---|---|---|
| Compact（自動） | 裁到 80% | < 裁剪點的 entries 移除 | 不動（已有全量） | 不動 |
| Reset（`/reset`） | 刪除 | 清除 | 清除 | 保留 |
| Remove session | 整個目錄刪 | 清除 | 清除 | 整個目錄刪 |

## 外部文件 RAG（KuraDB）

上述三層皆服務對話記憶——歷史對話、摘要、錯誤紀錄。要查詢**使用者自管的文件集**（筆記、收件匣、程式碼倉……），Agenvoy 委派給 [KuraDB](KuraDB-RAG.zh.md)——daemon 在 `kuradb_enabled=true` 時 spawn 的 in-process child。

KuraDB 對 agent 暴露兩個工具（`list_rag` / `search_rag`），endpoint 不存在時 per-turn 動態排除。載入時 system prompt 強制 information query 第一波先呼這兩者（外部 web 工具退為補足角色）。

這個切割是刻意的：ToriiDB + SQLite 是整合在 runtime 的記憶層（不能停用）；KuraDB 是 opt-in 的索引知識庫（透過 `/feature kuradb` TUI 啟用）。

## 遷移備註

Session 與 error memory 過去存在 per-session JSON 檔。從 ToriiDB v0.5.0 起改進 embedded store。**勿恢復 JSON 路徑**。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
