# 工具系統

> [English](https://github.com/agenvoy/Agenvoy/wiki/Tools)

## 內建工具

### 檔案操作

| 工具 | 說明 |
|---|---|
| `read_file` | 讀取檔案（text／PDF 透過 `pdftotext`／PPTX slide-level／DOCX line-level／CSV-TSV → JSON 2D 陣列／image 副檔名 error 導向 `read_image`） |
| `read_image` | 讀取影像為 base64 + metadata，給支援 vision 的 model |
| `write_file` | 建立或完整覆寫檔案 |
| `patch_file` | 精準字串替換（支援 `replace_all`） |
| `list_files` | 列出目錄 |
| `glob_files` | glob pattern 搜尋 |
| `search_content` | 檔案內容 regex 搜尋 |

### Web（read-only，多數可並發）

| 工具 | 並發 | 說明 |
|---|---|---|
| `fetch_page` | ✓ | 抓網頁（readability + 4xx/5xx skip cache 經 ToriiDB） |
| `save_page_to_file` | | 抓網頁後存檔 |
| `search_web` | | DuckDuckGo lite endpoint，package 層 rate limit（2 s gap） |
| `fetch_google_rss` | ✓ | Google News RSS |
| `fetch_yahoo_finance` | ✓ | 股票／財務數據 |
| `fetch_youtube_transcript` | ✓ | YouTube 字幕 |
| `send_http_request` | ✓ | 原始 HTTP 請求，回 status + headers + body |
| `calculator` | ✓ | 數學表達式求值 |

### Agent 編排

| 工具 | 說明 |
|---|---|
| `invoke_subagent` | In-process subagent（不走 HTTP）；支援 `name` / `session_id` / `model` / `system_prompt` / `exclude_tools` |
| `invoke_external_agent` | 一次性外部 CLI（claude / codex / copilot / gemini）；`readonly` 旗標控制寫入權限 |
| `cross_review_with_external_agents` | 串四家外部 CLI 互審至三輪上限（`MaxVerifyRounds=3`，package 常數） |
| `review_result` | 內部優先 model 自審 |
| `ask_user` | free-text／single-select／multi-select／`secret` 遮罩輸入；`pending` 啟用時走 registry，否則 fallback 至 stdin（CLI）或非互動引導訊息 |
| `store_secret` | 透過遮罩輸入取值並直接寫 keychain —— **value 從未進入 LLM context、history 或 log** |

### 記憶

| 工具 | 說明 |
|---|---|
| `search_conversation_history` | 當前 session 歷史的 keyword + semantic 並聯 |
| `remember_error` | 記錄工具錯誤與解法／策略 |
| `search_error_memory` | 跨 session 的 error memory 語意搜尋 |
| `read_error_memory` | 依 key 讀取指定錯誤紀錄 |

### Skill 探索

| 工具 | 說明 |
|---|---|
| `activate_skill` | 載入 skill 進當前迴圈（合成 tool_call/tool_result pair 注入 ToolHistories） |
| `search_tools` | 搜尋已註冊 tool 目錄 |
| `list_tools` | 列所有 tool |

### 系統

| 工具 | 說明 |
|---|---|
| `run_command` | 執行系統指令（argv-only schema，經 `go-pkg/sandbox` 包裝）；`cd` 特殊化直接 mutate `Executor.WorkDir`，不走 sandbox |

### 排程

| 工具 | 說明 |
|---|---|
| `add_task` / `get_task` / `list_tasks` / `update_task` / `remove_task` | 一次性排程任務 |
| `add_cron` / `get_cron` / `list_crons` / `update_cron` / `remove_cron` | 週期 cron 任務 |
| `read_script` / `update_script` | 排程腳本讀寫 |

TUI 模式下 `internal/tui/schedulerMonitor.go` 用 fsnotify 監看 scheduler 目錄，JSON 檔變動即熱重載 task／cron；過期 task 在啟動或重載時自動補跑。

## 工具擴展

### Script 工具（`script_*`）

把 Python／Node.js／shell script 放 `extensions/scripts/<name>/`，附 `tool.json` descriptor。Agenvoy 啟動時自動註冊為 `script_<name>`。

```
extensions/scripts/my-tool/
├── tool.json     # name、description、parameter schema、command
└── run.py        # 實際腳本
```

### API 工具（`api_*`）

把描述 REST endpoint 的 JSON 放 `extensions/apis/<name>.json`，自動註冊為 `api_<name>`。每個 `api_<name>` 有 per-name 1 s rate limiter（`reserveAPISlot`）。

> **Confirm gate** —— `api_*` tool **不**在 prefix 豁免名單內。使用者可能定義 destructive endpoint（DELETE／POST 寫入），`agen cli` 會逐個 confirm；要批次自動放行用 `agen run`。

### MCP 工具（`mcp__*`）

MCP server 暴露的 tool 自動註冊為 `mcp__<server>__<tool>`。配置見 [MCP 整合](https://github.com/agenvoy/Agenvoy/wiki/MCP-整合)。MCP tool 輸出每次上限 **1 MiB**，避免超過 provider 上限。

## 工具設計原則

新增／編輯 tool 必對照（由 `/tool-reviewer` 稽核）：

1. **Name 為唯一語意載具** —— stub tool 首呼叫只看 name，description／params 第二輪才進 context
2. **Description 只服務參數呼叫正確性** —— 不寫使用手冊／觸發條件／與他 tool 比較
3. **一律英文** —— 中文僅出現於面向使用者的 handler return 訊息
4. **Optional 欄位必須帶 `default`** —— Handler 仍須對 nil／missing 防禦

Description 長度：預設**單句**動詞開頭。**禁止**：觸發條件（「Use when ...」）、tool 選用比較、後續流程指示、輸出 schema 細節。

## 工具併發標記

每個工具有兩個獨立旗標：

- `ReadOnly` —— `agen cli` 模式下豁免 confirm
- `Concurrent` —— 加入 Pass 2 fan-out（每筆呼叫一個 goroutine）

要標 `Concurrent: true` 須同時「無副作用」+「上游允許併發」。當前 concurrent 集合見 [核心概念](https://github.com/agenvoy/Agenvoy/wiki/核心概念#三段式工具併發)。

## 憑證自動修復

`store_secret` 是 `AlwaysLoad: true`，agent 第一輪即可看到。當下游 tool 回傳 missing-key 或 invalid credential（`401`／`403`／`invalid api key`／`expired token`），system prompt `§10 Credential auto-heal` SOP 引導 agent 呼叫 `store_secret`（透過遮罩輸入取新值 —— value 從未進 LLM）後 retry 原 tool。每個 failing tool per turn 上限 2 輪 `store_secret`。
