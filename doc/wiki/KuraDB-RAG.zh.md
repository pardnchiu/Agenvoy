# KuraDB RAG

> [English](KuraDB-RAG.md)

KuraDB 是 Agenvoy 的 in-process RAG（Retrieval-Augmented Generation）提供者。以 daemon 管理的 child process 形式運行，對 agent 暴露兩個搜尋工具（`list_rag`、`search_rag`）。

## 是什麼

KuraDB（[pardnchiu/KuraDB](https://github.com/pardnchiu/KuraDB)）是一個自研的本機文件索引：

- 把使用者檔案（筆記、收件匣、程式碼……）索引進多個具名資料庫
- 透過 `gse` 分詞做關鍵字搜尋（支援中文）
- 透過 OpenAI embeddings（`text-embedding-3-small`）做語意搜尋
- 完全本機運行——無外部服務

Agenvoy 透過本機 HTTP API 與 KuraDB 溝通（KuraDB 啟動時把隨機 port 寫到 `~/.config/kuradb/endpoint`）。

## 生命週期

KuraDB 程式碼住在 `internal/runtime/kuradb/`：

| 檔案 | 職責 |
|---|---|
| `kuradb.go` | 公開介面：`BinaryPath`、`EndpointExists()`、`ReadEndpoint()`、`BinaryInstalled()`、`HasOpenAIKey()`、`SetOpenAIKey()` |
| `run.go` | `RunChild(ctx)`——`exec.Cmd` start + `StdoutPipe`/`StderrPipe` → slog；5 秒 crash backoff；health check goroutine 每分鐘 poll `<endpoint>/api/health`（5s timeout），連續 3 次失敗 → auto-disable |

### Daemon 編排（`cmd/app/cmdDeamon.go::reloadKuradb`）

Daemon 透過 fsnotify 監聽 `~/.config/agenvoy/config.json` 變動：

1. config 變動且 `kuradb_enabled=true` 時，spawn **之前**三道 gate：
   - `kuradb.BinaryInstalled()`——`/usr/local/bin/kura` 必須存在
   - `kuradb.HasOpenAIKey()`——`OPENAI_API_KEY` 必須在 keychain（service `agenvoy`）
2. 任一 gate fail → **silent return**（使用者明確選擇此行為——「直接忽視比較實在」：不 log、不 auto-disable、不寫 config）
3. 通過 → `RunChild` spawn 子進程；KuraDB 把 endpoint URL 寫到 `~/.config/kuradb/endpoint`
4. Healthcheck goroutine 啟動；連續 3 次失敗 → 寫 `kuradb_enabled=false` + 刪 endpoint 檔 + **明確** `reloadKuradb()` 呼叫（不靠 fsnotify async，避 200ms race window）

### Crash 恢復

`RunChild` 把 child 包在 5 秒 backoff 迴圈。stdout/stderr 透過 `bufio.Scanner` 接到 `slog`，KuraDB error 會落在 `daemon.log`，不會被吞掉。

## Tool 註冊

兩個 RAG 工具住在 `internal/runtime/kuradb/tool/`，在三個入口（`cmd/app/{main,cmdDeamon,newTUI}.go`）以明確的 `kuradbTool.Register()` 呼叫註冊（不走 `init()`——`init()` 早於 `filesystem.Init()`，gate 檢查必 fail）。

| Tool | 說明 |
|---|---|
| `list_rag` | 列出可用的 KuraDB 資料庫（例：`notes`、`inbox`、`code`） |
| `search_rag` | 透過 `mode=keyword`（`gse` 分詞）或 `mode=semantic`（OpenAI embeddings）搜尋資料庫 |

Tool gate 為單一條件 `cfg.KuradbEnabled`——per-handler 內的 `ReadEndpoint()` 呼叫是第二道防線（萬一 endpoint 在 turn 中消失）。

## Per-turn 動態排除

`exec.Execute()` 在 `NewExecutor` 之後檢查 `kuradb.EndpointExists()`。為 false 時把兩個 RAG tool 加進 `data.ExcludeTools`，既有的 filter 機制把它們從 `exec.Tools` 中拔掉。

結果：endpoint down 時 LLM **完全看不到** `list_rag` / `search_rag` tool——連 stub 名都沒。system prompt 內條件式「when `list_rag` / `search_rag` tools are present」段落自然失效。

**為何重要：** 沒有動態排除的話，LLM 在 startup race（KuraDB child 還沒 spawn 之前）會看到 RAG tool stub、呼叫它、拿到 error——LLM 與使用者都被搞糊。

## `/kuradb` TUI wizard

enable／disable 只透過 TUI（**無** CLI 子命令——install.sh + sudo prompt 需要真正的 TTY）：

```
/kuradb         → popup: enable | disable
```

### Enable 流程

1. Wizard 檢查 `HasOpenAIKey()`；缺失時開 `popupText` 收 key → `keychain.Set("OPENAI_API_KEY", value)`（service：`agenvoy`）
2. `tea.ExecProcess` 跑安裝 script：
   ```
   curl -fsSL https://cloud.agenvoy.com/KuraDB/install.sh | bash
   ```
   TTY 還給 child，`sudo` prompt 與 package manager 輸出才能正常運作
3. 驗證 `/usr/local/bin/kura` 存在；寫 `kuradb_enabled=true` 到 config.json
4. Daemon 透過 fsnotify 接到 → `reloadKuradb()` spawn child → endpoint 檔出現 → 工具可呼叫

### Disable 流程

1. `tea.ExecProcess` 跑 `sudo rm /usr/local/bin/kura`
2. 寫 `kuradb_enabled=false` 到 config.json
3. Daemon `reloadKuradb()` 通知 running child shutdown

## RAG-first prompting

當 `list_rag` / `search_rag` 工具被載入時，base system prompt 要求：**任何 information query 的第一波 tool calls** 必為 `list_rag` + `search_rag`——外部 web／search 工具為**次要**（補足 RAG 沒命中的部分），不是 fallback 也不是替代。

規則寫在 `configs/prompts/system_prompt.md`；KuraDB off 時規則自動失效（因為 `list_rag` / `search_rag` 不會在 tool list 內）。

## 檔案與路徑

| 路徑 | 用途 |
|---|---|
| `/usr/local/bin/kura` | KuraDB binary（由 `install.sh` 安裝） |
| `~/.config/kuradb/endpoint` | 純文字 URL，KuraDB 啟動寫入、disable 移除 |
| `~/.config/kuradb/` | KuraDB 自身的 config／data 目錄（由 KuraDB 管理） |
| Keychain `agenvoy/OPENAI_API_KEY` | 與 Agenvoy 其他用到 OpenAI 的功能共用 |

## 相關頁面

- [工具系統](Tools.zh.md#rag) —— `list_rag` / `search_rag` 工具定義
- [記憶系統](Memory-System.zh.md) —— KuraDB 如何補足 ToriiDB-backed 對話記憶
- [命令列參考](CLI-Reference.zh.md) —— `/kuradb` TUI 指令
- [設定檔](Configuration.zh.md#kuradb) —— config 鍵與路徑

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
