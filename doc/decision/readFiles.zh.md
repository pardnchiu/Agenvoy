# read_files：有無 decision model 的比較

最後更新：2026-11-22

> 回到 [文件](../doc.zh.md)

本文比較 `read_files` 在沒有 decision model 與加入 decision model 篩選後的實測差異，聚焦三件事：

1. **相關性**：與任務無關的檔案是否被擋下
2. **內容篩選**：與任務無關的段落是否在回傳前被篩掉，且關鍵內容仍保留
3. **內容減量**：實際交給 agent 的內容少了多少

## 摘要

| 指標 | 無 | 有 |
| --- | --- | --- |
| 無關檔案被擋下 | 0 / 12 | **12 / 12** |
| 相關檔案被誤擋 | 0 / 12 | **0 / 12** |
| 相關任務保留關鍵內容 | 12 / 12 | **12 / 12** |
| 相關任務送出的內容（342 KB） | 342 KB | **189 KB，少 44.8%** |
| 相關任務送出的內容，16K 字元以上的檔案（288 KB） | 288 KB | **134 KB，少 53.3%** |
| 無關任務送出的內容（342 KB） | 342 KB | **2.2 KB**（僅通知） |
| 每次讀取平均耗時（24 次） | 19 ms | 245 ms |

沒有 decision model 時，不論任務為何，每次讀取都回傳完整檔案。加入後，無關的檔案只回一則簡短通知，相關的長檔案拿掉與任務無關的段落，相關的短檔案照常整檔回傳。

## 兩種模式的運作方式

| | 無 | 有 |
| --- | --- | --- |
| 16K 字元以下的檔案 | 整檔回傳 | 整檔判斷一次；無關時以未載入通知取代，相關時整檔回傳 |
| 16K 字元以上的檔案 | 整檔回傳 | 依空行切成段落逐段判斷；無關段落以行號範圍標記取代，沒有任何相關段落時以未載入通知取代整檔 |
| 取得完整檔案 | 一律完整 | 帶 `full: true` 重讀，跳過篩選 |
| 篩選後寫檔 | — | `write_file` 拒絕仍含標記的內容，要求以 `full: true` 重讀 |

標記會寫出被取代的行號，例如 `[unrelated: rows 120-188 not loaded; read it again with full: true to load the whole file]`，agent 看得到缺口，也知道怎麼補讀。段落判斷失敗或逾時時保留該批段落；未設定 decision model 時，每次讀取都與「無」欄相同。

## 1. 相關性：無關檔案是否被擋下

每個檔案搭配一個相關任務與一個無關任務（`How do I bake sourdough bread at home?`）。相關任務不應被擋下，無關任務應收到未載入通知。

| 檔案 | 大小 | 路徑 | 相關 | 無關 |
| --- | --- | --- | --- | --- |
| `README.md` | 26 KB | 逐段 | 回傳 | 擋下 |
| `doc/doc.md` | 135 KB | 逐段 | 回傳 | 擋下 |
| `doc/architecture.md` | 39 KB | 逐段 | 回傳 | 擋下 |
| `internal/agents/exec/toolCall.go` | 34 KB | 逐段 | 回傳 | 擋下 |
| `internal/agents/exec/execute.go` | 34 KB | 逐段 | 回傳 | 擋下 |
| `internal/runtime/tui/viewRender.go` | 20 KB | 逐段 | 回傳 | 擋下 |
| `CLAUDE.md` | 24 KB | 整檔 | 回傳 | 擋下 |
| `configs/configs.go` | 10 KB | 整檔 | 回傳 | 擋下 |
| `internal/tools/file/boundary/boundary.go` | 8.2 KB | 整檔 | 回傳 | 擋下 |
| `internal/tools/file/relevance.go` | 5.0 KB | 整檔 | 回傳 | 擋下 |
| `go.mod` | 4.7 KB | 整檔 | 回傳 | 擋下 |
| `makefile` | 1.8 KB | 整檔 | 回傳 | 擋下 |

大小為 `read_files` 輸出的位元組數，含行號前綴。`CLAUDE.md` 雖有 24 KB，但內容多為中文，字元數低於 16K，因此走整檔判斷。

## 2. 內容篩選：回傳前是否篩掉無關段落

僅計相關任務。每個任務列出一項必須出現在結果中才算保留的關鍵內容。

| 檔案／任務 | 關鍵內容 | 有 | 標記數 |
| --- | --- | --- | --- |
| `README.md`／如何安裝 Agenvoy | `install.sh` | 保留 | 14 |
| `doc/doc.md`／decision model 呼叫逾時 | `times out after 3 seconds` | 保留 | 17 |
| `doc/architecture.md`／注入哪幾層 `official_guides` | `_vendor_` | 保留 | 6 |
| `toolCall.go`／寫檔後 `read_files` 快取如何失效 | `invalidateReadFileCache` | 保留 | 12 |
| `execute.go`／`alreadyCall` 何時重置 | `alreadyCall = make` | 保留 | 10 |
| `viewRender.go`／TUI 如何顯示 `write_file` diff | `writeDiffLine` | 保留 | 9 |
| `CLAUDE.md`／Go build 為何要帶 `-tags fts5` | `FTS5` | 保留，整檔 | — |
| `configs.go`／`MAX_DECISION_STATE_RUNES` 的值 | `20_000` | 保留，整檔 | — |
| `boundary.go`／`ResolveIn` 如何解析路徑 | `func ResolveIn` | 保留，整檔 | — |
| `relevance.go`／相關性篩選的大小門檻 | `MIN_RELEVANCE_RUNES` | 保留，整檔 | — |
| `go.mod`／使用哪個 `go-llm-router` 版本 | `go-llm-router v` | 保留，整檔 | — |
| `makefile`／binary 如何建置 | `fts5` | 保留，整檔 | — |

- 12 項關鍵內容全數保留。6 個長檔案逐段篩選，各留下 6–17 個標記；6 個短檔案整檔回傳。
- 沒有 decision model 時 12 項同樣都在，因為內容從不被移除；差別在於一併送出的內容量。

## 3. 內容減量

基準為 `full: true` 的輸出，也就是沒有 decision model 時每次讀取的結果。僅計相關任務；無關任務一律只回約 170–210 B 的通知。

| 檔案 | 無 | 有 | 減量 |
| --- | --- | --- | --- |
| `README.md` | 26 KB | 9.6 KB | 63.3% |
| `doc/doc.md` | 135 KB | 59 KB | 56.7% |
| `doc/architecture.md` | 39 KB | 28 KB | 27.1% |
| `internal/agents/exec/toolCall.go` | 34 KB | 15 KB | 56.9% |
| `internal/agents/exec/execute.go` | 34 KB | 13 KB | 61.8% |
| `internal/runtime/tui/viewRender.go` | 20 KB | 10 KB | 47.8% |
| 16K 字元以下的 6 個檔案 | 55 KB | 55 KB | 0% |
| **合計** | **342 KB** | **189 KB** | **44.8%** |

- 以約 4 bytes／token 估算：無 decision model 約 86K tokens，有 decision model 約 47K tokens。
- `doc/architecture.md` 減量最少，因為段落長，任務又涉及其中好幾段。
- `doc/doc.md` 仍回傳 59 KB，是篩選後最大的輸出；135 KB 的檔案中有許多段落與任務相關。

## 耗時

| 範圍 | 無（平均） | 有（平均） | 差異 |
| --- | --- | --- | --- |
| 16K 字元以上的檔案（12 次） | 28 ms | 289 ms | +261 ms |
| 16K 字元以下的檔案（12 次） | 10 ms | 201 ms | +191 ms |

沒有 decision model 時只是讀本機檔案。多出的時間來自 decision 呼叫：短檔案呼叫一次，長檔案的段落分批並行判斷。

## 方法與限制

- 版本：git `bb0de320`。
- 兩種模式都在同一個測試程序中，經由工具入口 `toolregister.Dispatch("read_files")` 呼叫：「無」帶 `full: true`，跳過篩選的效果與未設定 decision model 相同；「有」帶 `full: false`。decision model 為 `typesafe@jev-latest`，兩者依序執行。
- 每個案例只跑一次。decision model 的回答在不同次執行間可能不同，邊界段落可能時有時無。
- 關鍵內容以字串比對檢查，只能證明內容存在，不能證明周邊脈絡足夠。樣本為本 repo 的 12 個檔案，偏重 Go 原始碼與英文文件。
- 基準為含行號前綴的 `read_files` 輸出，以預設的 2,048 行上限讀取。
