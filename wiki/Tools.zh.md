# 工具系統

> [English](Tools.md)

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
| `transcribe_media` | ✓ | 本地音訊／影片轉逐字稿，走 Gemini `inline_data`（ogg、mp3、wav、m4a、flac、aac、mp4、mov、webm、mpeg、3gp）；單檔上限 20 MiB，與 Telegram `Bot.Save` 對齊 |
| `send_http_request` | ✓ | 原始 HTTP 請求，回 status + headers + body |
| `calculator` | ✓ | 數學表達式求值 |

### Agent 編排

| 工具 | 說明 |
|---|---|
| `invoke_subagent` | In-process subagent（不走 HTTP）；支援 `name` / `session_id` / `model` / `system_prompt` / `exclude_tools`。強制排除集：`invoke_subagent` 自身、`invoke_external_agent`、`cross_review_with_external_agents`、`review_result`。`AllowAll` 與 `WorkDir` 從父 ctx 繼承 |
| `invoke_external_agent` | 一次性外部 CLI（claude / codex / copilot / gemini）；`readonly` 旗標控制寫入權限。Subprocess timeout 由 `MAX_EXTERNAL_AGENT_TIMEOUT_MIN`（default 10 分）封頂 |
| `cross_review_with_external_agents` | 串四家外部 CLI 互審至三輪上限（`MaxVerifyRounds=3`，package 常數）。15 分硬上限 |
| `review_result` | 內部優先 model 自審 |
| `generate_plan` | 回傳結構化 markdown 計畫（需求總結／前置／步驟+驗收／整體驗收／風險／回退）。走 `exec.SelectAgent(ctx, dispatcher, registry, "[plan] " + requirement, ...)` —— `[plan]` prefix 觸發 `agent_selector.md` P0.6 routing 挑強 reasoning agent（claude-opus > codex-pro > codex > claude-sonnet > ...）。送 agent 時 `toolDefs=nil`，planner 無 tool 可呼 —— plan only, no execution。5 分上限 |
| `ask_user` | free-text／single-select／multi-select／`secret` 遮罩輸入；`pending` 啟用時走 registry，否則 fallback 至 stdin（CLI）或非互動引導訊息 |
| `store_secret` | 透過遮罩輸入取值並直接寫 keychain —— **value 從未進入 LLM context、history 或 log**。Schema **不**收 `value` 參數；agent 只看到 `name` + description |

### 記憶

| 工具 | 說明 |
|---|---|
| `search_conversation_history` | 當前 session 歷史的 keyword + semantic 並聯 |
| `remember_error` | 記錄工具錯誤與解法／策略 |
| `search_error_memory` | 跨 session 的 error memory 語意搜尋 |
| `read_error_memory` | 依 key 讀取指定錯誤紀錄 |

### RAG

透過 KuraDB child process 的外部文件 RAG。生命週期／health check 見 [KuraDB RAG](KuraDB-RAG.zh.md)。`~/.config/kuradb/endpoint` 不存在時三個工具會**per-turn 動態排除**——LLM 完全看不到。

| 工具 | 說明 |
|---|---|
| `rag_list_db` | 列出可用的 KuraDB 資料庫（例：`notes`、`inbox`、`code`） |
| `rag_search_keyword` | 透過 `gse` 分詞做關鍵字搜尋（支援中文） |
| `rag_search_semantic` | 透過 OpenAI embeddings（`text-embedding-3-small`）做語意搜尋 |

當 `rag_*` 工具被載入時，system prompt 強制：任何 information query 的**第一波** tool calls 必為 `rag_list_db` + `rag_search_*`。外部 web／search 工具為次要（補足 RAG 沒命中的部分），非 fallback 也非替代。

### 渲染

| 工具 | 說明 |
|---|---|
| `update_page` | 覆寫當前 session canvas 的 HTML 頁；瀏覽器分頁透過 SSE 自動 reload |

### Channel

跨 session 推送工具與 channel format reference。各工具雙重 gate：`cfg.{T,D}Enabled` 與 keychain credential。

| 工具 | 說明 |
|---|---|
| `list_telegram_chat` | 列出已授權 Telegram chat（`id` + `name`）；讀取 `~/.config/agenvoy/.telegram`。*(需 telegram)* |
| `send_to_telegram_chat` | 依 `chat_id` 送 HTML 格式訊息。Transient client（非 daemon long-poll bot）。強制 `parse_mode=HTML`。*(需 telegram)* |
| `telegram_format` | `AlwaysLoad=true`；回傳完整 Telegram HTML formatting reference（允許 tag、escape 規則、檔案／語音 markers、字數上限）。*(需 telegram)* |
| `list_discord_channel` | 列出已授權 Discord channel（`id` + `name`）。*(需 discord)* |
| `send_to_discord_channel` | 依 `channel_id` 送 markdown 格式訊息。透過 daemon REST 走 transient client。*(需 discord)* |
| `discord_format` | `AlwaysLoad=true`；回傳 Discord markdown reference。*(需 discord)* |

### 輸出 markers（channel-specific 行為）

任何 tool 或 LLM 回應的輸出文字會被掃 marker 並對應行為：

| Marker | 行為 |
|---|---|
| `FILE: <path>`（單獨一行） | Channel runtime 自動 attach 檔案（Telegram → 依副檔名分 photo/document，Discord → 統一 `SendFiles`，10 個/訊息分批） |
| `[SEND_FILE:<path>]`（inline） | 同上 —— LLM 主動要求 attach 時用 |
| `[SEND_VOICE:<text>]` | 僅 Telegram。透過 Gemini TTS 合成 OGG voice 送出。Run.go **async** 觸發（`go func` + `context.WithoutCancel`），reply 文字立即 return。失敗 → `slog.Error` + chat notify `⚠️ SendVoice failed (background)`（不能靜默） |

Marker regex + dedupe + `os.Stat` 過濾的唯一住處在 `internal/utils/fileMarker.go`。Push hook（`telegram.PushTelegramResult` / `discord.PushDiscordResult`）共用同一 extractor —— cron fire 期間生圖也能正確 attach。

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
| `add_task` / `add_cron` | 把既有 scheduler skill 綁定至 one-shot 時間或 5 欄 cron expression。`add_task` time 格式：`+5m`（相對）／`HH:MM`（今天）／`YYYY-MM-DD HH:MM`／RFC3339。**應由 `scheduler-skill-creator` skill 呼叫，不直接呼**——直呼前提是 skill 已存在於 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`。 |
| `patch_task` / `patch_cron` | 依 `skill_name` 改時間；只動時間、不動 SKILL body。 |
| `remove_task` / `remove_cron` | 依 `skill_name` 取消；綁定的 scheduler skill 目錄一併搬到 `.Trash/`。 |

`scheduler-skill-creator` 是高階 skill：**建立** scheduler skill body 後呼叫 `add_task`／`add_cron` 完成綁定。新的週期／一次性任務需求應 activate 該 skill，而非直呼低階 tool。

Daemon 端 runtime（`internal/runtime/scheduler.go`）用 fsnotify 監看 `~/.config/agenvoy/{tasks,crons}.json`，Write／Create／Rename 觸發即熱重載；過期 task 在啟動或重載時自動觸發並移除；觸發走 `runtime.SetRunner` → 對 scheduler skill body 起 in-process subagent（always-allow context）。

TUI 提供三個 slash command 管排程：`/cron`、`/task`（add／remove／edit）、`/sched-<name>`（手動觸發既有 scheduler skill body）。Popup 流程見 [CLI Reference](CLI-Reference.zh)。

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

MCP server 暴露的 tool 自動註冊為 `mcp__<server>__<tool>`。配置見 [MCP 整合](MCP-Integration.zh.md)。MCP tool 輸出每次上限 **1 MiB**，避免超過 provider 上限。

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

要標 `Concurrent: true` 須同時「無副作用」+「上游允許併發」。當前 concurrent 集合見 [核心概念](Core-Concepts.zh.md#三段式工具併發)。

## Tool timeout 矩陣

各 adapter 自有 timeout，再加上 executor 層 ceiling：

| Adapter | Default | 可調 | 位置 |
|---|---|---|---|
| 內建（`toolRegister.Dispatch`） | 1 分 | per-tool `Def.Timeout` | tool 註冊處 |
| Script（`script_*`） | 5 分（300s） | `tool.json` `"timeout": <秒>` | `extensions/scripts/<name>/tool.json` |
| API（`api_*`） | 60s | `doc.Endpoint.Timeout`；硬上限 300s | `extensions/apis/<name>.json` |
| MCP HTTP | 60s `http.Client.Timeout` + 1 分外層 dispatch | n/a | MCP server config |
| MCP stdio | 僅 1 分外層 dispatch | n/a | MCP server config |

長跑 tool（script + API）每 30s 在 daemon log 印 `running name=... elapsed=Ys/Zs` 提供可見性。

Subagent + external-agent 工具另有多分鐘 cap（`invoke_subagent` = `MAX_SUBAGENT_TIMEOUT_MIN`、`invoke_external_agent` = 10 分、`cross_review_with_external_agents` = 15 分、`generate_plan` / `transcribe_media` = 5 分）。

## 憑證自動修復

`store_secret` 是 `AlwaysLoad: true`，agent 第一輪即可看到。當下游 tool 回傳 missing-key 或 invalid credential（`401`／`403`／`invalid api key`／`expired token`），system prompt `§10 Credential auto-heal` SOP 引導 agent 呼叫 `store_secret`（透過遮罩輸入取新值 —— value 從未進 LLM）後 retry 原 tool。每個 failing tool per turn 上限 2 輪 `store_secret`。
