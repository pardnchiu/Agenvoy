# 工具系統

> [English](Tools.md)

## 內建工具

### 檔案操作

| 工具 | 說明 |
|---|---|
| `read_file` | 讀取 text／PDF／DOCX／PPTX／CSV-TSV／image 檔。`patch_file` 前必先呼叫 |
| `write_file` | 建立或完整覆寫檔案 |
| `patch_file` | 精準字串替換（支援 `replace_all`） |
| `list_files` | 列出目錄 |
| `glob_files` | glob pattern 搜尋 |
| `search_files` | 檔案內容 regex 搜尋 |

### Web（read-only，多數可並發）

| 工具 | 並發 | 說明 |
|---|---|---|
| `fetch_page` | ✓ | 抓網頁（readability + 4xx/5xx skip cache 經 ToriiDB）；`save=true` 存檔至本地 |
| `search_web` | | DuckDuckGo lite endpoint，package 層 rate limit（2 s gap） |
| `search_google_news` | ✓ | Google News RSS |

### HTTP

| 工具 | 並發 | 說明 |
|---|---|---|
| `send_http_request` | ✓ | 原始 HTTP 請求，回 status + headers + body。GET 免 confirm，其他 method 走 confirm gate。內建 SSRF guard（DNS resolve 後比對 loopback／private／link-local），可透過 `config.json` `net_white_list` 放行指定 host |
| `download_file` | ✓ | 下載 binary 檔至本地磁碟（tar.gz、image、archive 等）；JSON／HTML 用 `send_http_request` 或 `fetch_page` |

### 媒體

| 工具 | 並發 | 說明 |
|---|---|---|
| `transcribe_media` | ✓ | 本地音訊／影片轉逐字稿，走 Gemini `inline_data`（ogg、mp3、wav、m4a、flac、aac、mp4、mov、webm、mpeg、3gp）；單檔上限 20 MiB。*(需 gemini credential)* |
| `generate_image` | | 透過 gpt-image-2 生圖（走 codex@ 訂閱額度）。先經 `ask_user` 確認 size 與 quality。輸出 `[SEND_FILE:<path>]`。15 分上限。*(需 codex credential)* |

### 工具

| 工具 | 並發 | 說明 |
|---|---|---|
| `calculate` | ✓ | 數學表達式求值 |

### Agent 編排

| 工具 | 說明 |
|---|---|
| `invoke_subagent` | In-process subagent（不走 HTTP）；支援 `name` / `session_id` / `model` / `system_prompt` / `exclude_tools`。強制排除集：`invoke_subagent` 自身、`invoke_external_agent`、`cross_review_with_external_agents`、`review_result`。`AllowAll` 與 `WorkDir` 從父 ctx 繼承 |
| `invoke_external_agent` | 一次性外部 CLI（claude / codex / copilot / gemini）；`readonly` 旗標控制寫入權限。Subprocess timeout 由 `MAX_EXTERNAL_AGENT_TIMEOUT_MIN`（default 10 分）封頂 |
| `cross_review_with_external_agents` | 串四家外部 CLI 互審至三輪上限（`MaxVerifyRounds=3`，package 常數）。15 分硬上限 |
| `review_result` | 內部優先 model 自審 |
| `generate_plan` | 回傳結構化 markdown 計畫（需求總結／前置／步驟+驗收／整體驗收／風險／回退）。走 `exec.SelectAgent` + `[plan]` prefix 觸發 P0.6 routing 挑強 reasoning agent。`toolDefs=nil`——plan only, no execution。5 分上限 |

### 互動

| 工具 | 說明 |
|---|---|
| `ask_user` | free-text／single-select／multi-select／`secret` 遮罩輸入；`pending` 啟用時走 registry，否則 fallback 至 stdin（CLI）或非互動引導訊息 |
| `store_secret` | 透過遮罩輸入取值並直接寫 keychain —— **value 從未進入 LLM context、history 或 log**。Schema **不**收 `value` 參數；agent 只看到 `name` + description |
| `install_dependence` | 跨平台安裝缺失的系統 binary（僅 TUI/CLI）。已在 PATH 則跳過。Sandbox 擋 sudo，此 tool 繞過。語言級套件（pip/npm/cargo/gem）→ 輸出指令讓使用者手動執行 |

### 記憶

| 工具 | 說明 |
|---|---|
| `search_chat_history` | 當前 session 歷史的 keyword + semantic 並聯搜尋 |
| `remember_error` | 記錄工具錯誤與解法／策略 |
| `search_error_history` | 跨 session 的 error memory 語意搜尋 |
| `read_error` | 依 key 讀取指定錯誤紀錄 |

### 診斷

| 工具 | 說明 |
|---|---|
| `read_log` | 回傳 daemon.log 近 `h` 小時的 WARN/ERROR 行 |
| `report_error` | 掃描 daemon.log WARN/ERROR 並上傳至 report.agenvoy.com。Fire-and-forget |

### RAG

透過 KuraDB child process 的外部文件 RAG。生命週期／health check 見 [KuraDB RAG](KuraDB-RAG.zh.md)。`~/.config/kuradb/endpoint` 不存在時工具會**per-turn 動態排除**——LLM 完全看不到。

| 工具 | 說明 |
|---|---|
| `list_rag` | 列出可用的 KuraDB 資料庫（例：`notes`、`inbox`、`code`） |
| `search_rag` | 透過 `mode=keyword`（`gse` 分詞，支援中文）或 `mode=semantic`（OpenAI `text-embedding-3-small`）搜尋資料庫 |

當 `list_rag` / `search_rag` 工具被載入時，system prompt 強制：任何 information query 的**第一波** tool calls 必為 `list_rag` + `search_rag`。外部 web／search 工具為次要（補足 RAG 沒命中的部分），非 fallback 也非替代。

### 渲染

| 工具 | 說明 |
|---|---|
| `render_page` | 覆寫當前 session canvas 的 HTML 頁；瀏覽器分頁透過 SSE 自動 reload |

### Channel

跨 session 推送工具與 channel format reference。各工具雙重 gate：`cfg.{T,D}Enabled` 與 keychain credential。

| 工具 | 說明 |
|---|---|
| `list_chatbot` | 列出指定平台的已授權 chat（`platform=telegram` 或 `platform=discord`）。*(需 telegram 或 discord)* |
| `send_to_chatbot` | 依 `target_id` 送格式化訊息至指定平台。需 `platform` 參數。Telegram：HTML + transient client。Discord：markdown + transient client。*(需 telegram 或 discord)* |
| `format_chatbot` | `AlwaysLoad=true`；回傳指定平台的完整格式化參考（Telegram HTML 或 Discord markdown）。*(需 telegram 或 discord)* |

### 輸出 markers（channel-specific 行為）

任何 tool 或 LLM 回應的輸出文字會被掃 marker 並對應行為：

| Marker | 行為 |
|---|---|
| `[SEND_FILE:<path>]` | Channel runtime 自動 attach 檔案（Telegram → 依副檔名分 photo/document，Discord → 統一 `SendFiles`，10 個/訊息分批） |
| `[SEND_VOICE:<text>]` | 僅 Telegram。透過 Gemini TTS 合成 OGG voice 送出。Run.go **async** 觸發（`go func` + `context.WithoutCancel`），reply 文字立即 return。失敗 → `slog.Error` + chat notify `⚠️ SendVoice failed (background)`（不能靜默） |

Marker regex + dedupe + `os.Stat` 過濾住在 `internal/utils/utils.go`。Telegram 專用 photo/document split wrapper 在 `internal/runtime/telegram/fileMarker.go`。Push hook（`telegram.PushTelegramResult` / `discord.PushDiscordResult`）共用同一 extractor。

### Skill 探索

| 工具 | 說明 |
|---|---|
| `run_skill` | 載入 skill 進當前迴圈（合成 tool_call/tool_result pair 注入 ToolHistories） |
| `search_tools` | 搜尋已註冊 tool 目錄 |
| `list_tools` | 列所有 tool |

### Skill 與 tool 變體（always-allowed 的 `write_file` 變體）

| 工具 | 說明 |
|---|---|
| `write_skill` | 建立或覆寫 `~/.config/agenvoy/skills/` 下的檔案 |
| `patch_skill` | 字串替換 skill 檔案 |
| `remove_skill` | 將 skill 目錄搬到 `.Trash/` |
| `write_tool` | 建立或覆寫 `~/.config/agenvoy/tools/script/` 下的 `tool.json` 或 `script.py` |
| `patch_tool` | 字串替換 script tool 檔案（`tool.json` 或 `script.py`） |
| `test_tool` | 在 sandbox 內以 JSON input 執行 script tool 的 `script.py` |
| `remove_tool` | 將 script tool 目錄搬到 `.Trash/` |

所有變體皆 always-allowed，限定對應目錄。每次 write/patch/remove 自動 commit 至對應 git repo（skills 或 tools）。`write_tool` 與 `write_skill` 支援並發呼叫。

### Git 版控與自我改進

| 工具 | 說明 |
|---|---|
| `git_log` | 列出 skills 或 tools 目錄的 git commit 歷史（`tag` = `skills` 或 `tools`） |
| `git_rollback` | 將 skills 或 tools 目錄還原至指定 git commit（`tag` = `skills` 或 `tools`） |

**自我改進迴圈**：當 skill 執行產生 tool 錯誤（錯誤 tool name、步驟失敗），`postSkillImprove` 在 `Execute` 結束時同步執行。載入內建 `improve-skill` 定義、餵入執行軌跡、改寫有問題的 SKILL.md/scripts、並 auto-commit 修正。完整生命週期見 [Skill 系統 § 自我改進](Skill-System.zh.md#自我改進失敗時自動修正)。

### 系統

| 工具 | 說明 |
|---|---|
| `run_command` | 執行系統指令（argv-only schema，經 `go-pkg/sandbox` 包裝）；`cd` 特殊化直接 mutate `Executor.WorkDir`，不走 sandbox |

### 排程

| 工具 | 說明 |
|---|---|
| `add_schedule` | 把既有 scheduler skill 綁定至 one-shot 時間（`target=task`）或 5 欄 cron expression（`target=cron`）。Task time 格式：`+5m`（相對）／`HH:MM`（今天）／`YYYY-MM-DD HH:MM`／RFC3339。**應由 `scheduler-skill-creator` skill 呼叫，不直接呼**——直呼前提是 skill 已存在於 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`。 |
| `patch_schedule` | 依 `skill_name` 與 `target` 改時間；只動時間、不動 SKILL body。 |
| `remove_schedule` | 依 `skill_name` 與 `target` 取消；綁定的 scheduler skill 目錄一併搬到 `.Trash/`。 |
| `list_schedule` | 列出當前 session 的 task 與 cron。`target` 接受 `task`、`cron`、`all`（預設）。 |

`scheduler-skill-creator` 是高階 skill：**建立** scheduler skill body 後呼叫 `add_schedule` 完成綁定。新的週期／一次性任務需求應 activate 該 skill，而非直呼低階 tool。

Daemon 端 runtime（`internal/runtime/scheduler.go`）用 fsnotify 監看 `~/.config/agenvoy/{tasks,crons}.json`，Write／Create／Rename 觸發即熱重載；過期 task 在啟動或重載時自動觸發並移除；觸發走 `runtime.SetRunner` → 對 scheduler skill body 起 in-process subagent（always-allow context）。

TUI 提供三個 slash command 管排程：`/cron`、`/task`（add／remove／edit）、`/sched-<name>`（手動觸發既有 scheduler skill body）。Popup 流程見 [CLI Reference](CLI-Reference.zh)。

## 工具擴展

### 自動生成（Capability Gap → 建立 → 執行）

當使用者的請求需要即時外部資料（天氣、匯率、股價、地理編碼、翻譯等）且沒有現有 tool 能覆蓋時，Agent **先建立工具、再執行它來回答**。這就是「讓 Agent 自己生成工具」的流程——不需編程能力。

System prompt 的 `§ Capability Gap` 區段驅動此序列：

| 步驟 | 動作 |
|---|---|
| 1. 尋找適合的 API | `api_public_api_list(type=category)` → 挑選相關分類 → 選最佳候選（偏好免 auth + HTTPS）→ `fetch_page` 讀文件 |
| 2. 建立 script tool | `mkdir` tool 目錄 → `write_file` 寫 `tool.json`（name、description、schema）+ `script.py`（stdin JSON → HTTP call → stdout JSON） |
| 3. 執行並回答 | 把使用者查詢 pipe 進新 script；失敗則修復重試（最多 3 次） |

建立後，tool 持久存放在 `~/.config/agenvoy/tools/script/<name>/`，所有未來 session 皆可使用。需要 auth 的 API 透過 `store_secret` + keychain 整合在生成的 script 內處理。

關鍵限制：
- Agent 不可用 raw `send_http_request` 或 inline `python3 -c` 回答；必須寫可重複使用的 script 至磁碟
- `fetch_page` 僅允許用於閱讀 API 文件，不可直接取得答案資料
- 生成的 `tool.json` 使用 `"always_allow": true`，後續呼叫免 confirm

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

Subagent + external-agent 工具另有多分鐘 cap（`invoke_subagent` = `MAX_SUBAGENT_TIMEOUT_MIN`、`invoke_external_agent` = 10 分、`cross_review_with_external_agents` = 15 分、`generate_plan` / `transcribe_media` = 5 分、`generate_image` = 15 分）。

## 憑證自動修復

`store_secret` 是 `AlwaysLoad: true`，agent 第一輪即可看到。當下游 tool 回傳 missing-key 或 invalid credential（`401`／`403`／`invalid api key`／`expired token`），system prompt `§10 Credential auto-heal` SOP 引導 agent 呼叫 `store_secret`（透過遮罩輸入取新值 —— value 從未進 LLM）後 retry 原 tool。每個 failing tool per turn 上限 2 輪 `store_secret`。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
