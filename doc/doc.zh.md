# Agenvoy - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

### 系統需求

- Go 1.25 或更新版本
- 至少一組 AI provider 憑證（GitHub Copilot 訂閱或任一 API key）
- Discord Bot Token（僅 Discord 模式需要）

### Sandbox 依賴

| 平台 | 依賴 | 說明 |
|------|------|------|
| Linux | `bubblewrap` (`bwrap`) | 啟動時自動偵測，未安裝時會透過 `apt-get` / `dnf` / `yum` / `pacman` / `apk` 自動安裝 |
| macOS | `sandbox-exec` | macOS 內建，無需安裝 |

### 瀏覽器依賴（選用）

- Chromium 或 Google Chrome — 由 `fetch_page` / `save_page_to_file` 以 headless 模式使用
- 若系統沒有 Chrome，`go-rod` 會在首次使用時自動下載 Chromium

### Go 相依

| 套件 | 用途 |
|------|------|
| `github.com/bwmarrin/discordgo` | Discord Bot API |
| `github.com/gin-gonic/gin` | REST API server（HTTP 路由） |
| `github.com/go-rod/rod` | Headless Chrome 瀏覽器自動化 |
| `github.com/go-shiori/go-readability` | HTML 內容擷取與清理 |
| `github.com/joho/godotenv` | `.env` 環境變數載入 |
| `github.com/manifoldco/promptui` | 互動式 CLI 選單 |
| `github.com/pardnchiu/ToriiDB` | 嵌入式 KV 儲存（session 歷史、錯誤記憶、web 快取） |
| `github.com/pardnchiu/go-scheduler` | Cron 運算式解析與排程 |
| `github.com/rivo/tview` | Terminal UI 框架 |
| `github.com/gdamore/tcell/v2` | Terminal cell / event library |
| `github.com/fsnotify/fsnotify` | TUI 檔案監視 |
| `golang.org/x/net` | HTML tokenizer / 網路工具 |

## 安裝

### 使用 go install

```bash
go install github.com/pardnchiu/agenvoy/cmd/app@latest
```

### 從原始碼建置並安裝

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
make build  # 建置為 agen 並複製到 /usr/local/bin/agen
```

### 從原始碼直接執行（無需全域安裝）

```bash
make app                # 啟動 TUI + Discord + REST API
make run <input...>     # 執行 agent（自動核准所有工具）
make cli <input...>     # 執行 agent（工具逐一確認）
```

## 設定

### 新增 Provider

執行互動式設定以從嵌入 registry 選擇 provider 與模型：

```bash
agen add
```

支援的 provider：

| Provider | 認證方式 | 預設模型 |
|----------|---------|---------|
| GitHub Copilot | OAuth Device Code Flow（自動刷新） | `gpt-4.1` |
| OpenAI | API Key（keychain） | `gpt-5-mini` |
| OpenAI Codex | OAuth Device Code Flow（自動刷新） | `gpt-5.3-codex` |
| Claude | API Key（keychain） | `claude-sonnet-4-5` |
| Gemini | API Key（keychain） | `gemini-2.5-pro` |
| NVIDIA | API Key（keychain） | `openai/gpt-oss-120b` |
| Compat | 選用 API Key（keychain） | 使用者自訂 |

### 環境變數

| 變數 | 必要 | 說明 |
|------|------|------|
| `DISCORD_TOKEN` | 是（Discord 模式） | Discord Bot Token |
| `DISCORD_GUILD_ID` | 否 | 將 slash command 註冊限制於特定 guild |
| `PORT` | 否 | REST API server 監聽埠（預設：`17989`） |
| `MAX_HISTORY_MESSAGES` | 否 | 送至 agent 的歷史訊息上限（預設：16） |
| `MAX_TOOL_ITERATIONS` | 否 | 單次請求的最大工具呼叫迭代次數（預設：16） |
| `MAX_SKILL_ITERATIONS` | 否 | 單次 skill 執行的最大工具呼叫迭代次數（預設：128） |
| `MAX_EMPTY_RESPONSES` | 否 | 連續空回應放棄的上限（預設：8） |
| `EXTERNAL_COPILOT` | 否 | GitHub Copilot 的外部 agent endpoint |
| `EXTERNAL_CLAUDE` | 否 | Claude 的外部 agent endpoint |
| `EXTERNAL_CODEX` | 否 | Codex 的外部 agent endpoint |

複製 `.env.example` 並填入對應值：

```bash
cp .env.example .env
```

> 名稱含 `.example` 的檔案（例如 `.env.example`）會繞過 env 前綴 deny 規則，可以直接讀取。

### API 擴充

將 JSON 檔案放入 `~/.config/agenvoy/api_tools/` 以新增自訂 API 工具。每個檔案定義一個可呼叫的工具，啟動時載入：

```json
{
  "name": "my_tool",
  "description": "What the agent sees when selecting this tool",
  "endpoint": {
    "url": "https://api.example.com/resource/{id}",
    "method": "GET",
    "content_type": "json",
    "timeout": 30
  },
  "auth": {
    "type": "bearer",
    "env": "MY_API_KEY"
  },
  "parameters": {
    "id": {
      "type": "string",
      "description": "Resource ID",
      "required": true
    }
  },
  "response": {
    "format": "json"
  }
}
```

| 欄位 | 必要 | 說明 |
|------|------|------|
| `name` | 是 | 註冊給 agent 的 snake_case 工具名稱 |
| `description` | 是 | 提供給 LLM 做工具挑選的說明 |
| `endpoint.url` | 是 | 目標 URL；`{param}` 佔位符於呼叫時替換 |
| `endpoint.method` | 是 | HTTP 方法：`GET`、`POST`、`PUT`、`DELETE`、`PATCH` |
| `endpoint.content_type` | 否 | `json`（預設）或 `form` |
| `endpoint.headers` | 否 | 靜態 header map |
| `endpoint.timeout` | 否 | 請求逾時秒數（預設 30） |
| `auth.type` | 否 | `bearer` 或 `apikey` |
| `auth.env` | 否 | 憑證所在的環境變數名稱 |
| `auth.header` | 否 | `apikey` 類型的 header 名稱（預設 `X-API-Key`） |
| `parameters` | 是 | 參數定義（flat map） |
| `response.format` | 否 | `json`（預設）或 `text` |

每個參數支援：`type`（`string` / `integer` / `number` / `boolean`）、`description`、`required`、`default`、`enum`。

#### 嵌入的公開 API 擴充

啟動時自動載入下列嵌入擴充：

| 擴充 | 類別 | 說明 |
|------|------|------|
| `nominatim` | Geocoding | OpenStreetMap 地理編碼與反向地理編碼 |
| `coingecko` | Finance | 加密貨幣價格與市場資料 |
| `wikipedia` | Data | Wikipedia 文章搜尋與內容 |
| `world-bank` | Data | World Bank 發展指標 |
| `usgs-earthquake` | Data | USGS 地震資料 |
| `themealdb` | Data | 食譜與料理資料庫 |
| `hackernews` | Data | Hacker News 頭條與項目 |
| `rest-countries` | Data | 國家資訊與 metadata |
| `exchange-rate` | Finance | 貨幣匯率 |
| `ip-api` | Network | IP 地理定位查詢 |
| `open-meteo` | Weather | 開源天氣預報 API |
| `youtube` | Media | YouTube 影片 metadata（標題、說明、頻道、時長） |

### Script 工具擴充

將包含 `tool.json` + `script.js` 或 `script.py` 的子目錄放入 `~/.config/agenvoy/script_tools/`（或 `<workdir>/.config/agenvoy/script_tools/`）。啟動時會掃描兩個路徑並以 `script_` 前綴註冊每個工具。

#### 內建安裝腳本

repo 附有跨平台安裝腳本：

```bash
# 安裝 Threads API 工具
bash install_threads.sh

# 安裝 yt-dlp 工具
bash install_youtube.sh
```

兩個腳本會偵測 OS、驗證 Python 與必要套件，並將工具複製到 `~/.config/agenvoy/script_tools/`。

| 內建工具 | Script | 說明 |
|----------|--------|------|
| `script_threads_get_quota` | Python | 取得 Threads API 用量配額 |
| `script_threads_publish_text` | Python | 發佈純文字（前置 500 字驗證） |
| `script_threads_publish_image` | Python | 發佈含字幕的圖片 |
| `script_threads_publish_carousel` | Python | 發佈多圖輪播 |
| `script_threads_refresh_token` | Python | 刷新長效 Threads token |
| `script_yt_dlp_info` | JS / Python | 取得影片 metadata |
| `script_yt_dlp_downloader` | Python | NFC 檔名下載影片 |

Script 工具目錄結構：

```
~/.config/agenvoy/script_tools/
└── my-tool/
    ├── tool.json       # 工具 manifest
    └── script.py       # 或 script.js
```

I/O 契約 — executor 將工具參數以 JSON 寫入 stdin 並從 stdout 讀取結果：

```python
#!/usr/bin/env python3
import json, sys

params = json.loads(sys.stdin.read() or "{}")
result = {"output": params.get("input", "").upper()}
print(json.dumps(result))
```

### Skill 擴充

Skill 擴充是帶 YAML frontmatter 的 Markdown 檔案。啟動時 `SyncSkills` 會從嵌入 FS 將 repo 內 `extensions/skills` 下尚未存在的 skill 目錄複製到 `~/.config/agenvoy/skills/`，再由 scanner 掃描 9 個標準路徑。

Skill 檔案格式：

```markdown
---
name: my-skill
description: One-line summary shown to the agent for skill selection
---

# My Skill

Instructions the agent follows when this skill is selected...
```

## 使用方式

### Make 指令

由 repo 根目錄執行：

| 目標 | 指令 | 說明 |
|------|------|------|
| `make build` | `go build -o agen ./cmd/app/ && sudo mv agen /usr/local/bin/agen` | 建置 binary 並安裝到 `/usr/local/bin/agen` |
| `make app` | `go run ./cmd/app/` | 啟動統一應用（TUI + Discord + REST API） |
| `make add` | `go run ./cmd/app/ add` | 互動式新增 provider / 模型 |
| `make remove` | `go run ./cmd/app/ remove` | 移除已設定的 provider |
| `make planner` | `go run ./cmd/app/ planner` | 設定 planner 模型 |
| `make reasoning` | `go run ./cmd/app/ reasoning` | 設定 reasoning level |
| `make models` | `go run ./cmd/app/ list` | 列出已設定模型 |
| `make skills` | `go run ./cmd/app/ list skill` | 列出可用 skill |
| `make cli <input...>` | `go run ./cmd/app/ cli <input>` | 執行 agent（工具逐一確認） |
| `make run <input...>` | `go run ./cmd/app/ run <input>` | 執行 agent（自動核准所有工具） |

### 基礎用法

啟動 TUI 應用（預設行為，無參數）：

```bash
agen
```

列出已設定模型：

```bash
agen list
```

列出可用 skill：

```bash
agen list skill
```

以互動模式執行 agent（每次工具呼叫前確認）：

```bash
agen cli "analyze the architecture of this project"
```

### 進階用法

自動核准模式（跳過所有確認提示）：

```bash
agen run "generate and write the README documentation"
```

移除 provider：

```bash
agen remove
```

設定 planner（路由）模型：

```bash
agen planner
```

## 命令列參考

### 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| `(無)` | `agen` | 啟動統一應用（TUI + Discord + REST API） |
| `add` | `agen add` | 互動式註冊 AI provider |
| `remove` | `agen remove` | 移除已設定的 provider |
| `planner` | `agen planner` | 設定 planner（路由）模型 |
| `reasoning` | `agen reasoning` | 設定某 provider 的 reasoning level |
| `list` | `agen list [skill]` | 列出已設定模型或可用 skill |
| `cli` | `agen cli <input...>` | 執行 agent，工具逐一確認 |
| `run` | `agen run <input...>` | 執行 agent，所有工具呼叫自動核准 |

### TUI 鍵盤快捷鍵

| 按鍵 | 模式 | 說明 |
|------|------|------|
| `:` | Normal | 進入命令輸入模式 |
| `Esc` | Command | 離開命令輸入模式 |
| `h` / `j` / `k` / `l` | Normal | vim 風格方向導覽 |
| `Ctrl+C` | 任一 | 結束 TUI |

### 內建工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `search_tools` | `query`, `max_results` | 按需搜尋並注入工具；支援 `select:<name>` 直接啟用、keyword fuzzy search 與 `+term` 必要關鍵字語法 |
| `read_file` | `path`, `offset`, `limit` | 讀取檔案內容；偵測並拒絕 binary；PDF 依副檔名分派（按頁分頁）；CSV／TSV 輸出 JSON 2D 陣列 `[[header...], [row1...], ...]`（剝除 BOM、永遠帶回 header、依 header 欄寬對齊） |
| `read_image` | `path` | 將本地圖片（JPEG/PNG/GIF/WebP，最大 10 MB）讀成 base64 JPEG data URL |
| `write_file` | `path`, `content` | 以 atomic write 寫入或建立檔案 |
| `list_files` | `path`, `recursive` | 列出目錄內容 |
| `glob_files` | `pattern` | Glob 比對（例如 `**/*.go`） |
| `search_content` | `pattern`, `file_pattern` | 以 regex 搜尋檔案內容 |
| `patch_file` | `path`, `old_string`, `new_string` | 首次命中字串替換（比完整重寫安全） |
| `search_conversation_history` | `keyword`, `time_range` | 在 ToriiDB 中查詢當前 session 的歷史紀錄 |
| `read_error_memory` | `hash` | 以 hash 取回失敗工具呼叫的完整錯誤細節 |
| `remember_error` | `tool_name`, `keywords`, `symptom`, `action` | 將錯誤解決方案寫入錯誤知識庫 |
| `search_error_memory` | `keyword` | 查詢錯誤知識庫 |
| `fetch_yahoo_finance` | `symbol`, `interval`, `range` | 取得 Yahoo Finance 報價與 OHLCV；query1/query2 並行，回傳最快者 |
| `fetch_youtube_transcript` | `url` | YouTube 影片逐字稿（含時間戳） |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google News RSS，含去重 |
| `send_http_request` | `method`, `url`, `headers`, `body` | 通用 HTTP 請求 |
| `search_web` | `query`, `time_range` | DuckDuckGo lite endpoint 網頁搜尋；`time_range` 僅接受 `1d` / `7d` / `1m` / `1y` |
| `fetch_page` | `url` | 以 headless Chrome 取得 JS 渲染後的頁面並轉為 Markdown |
| `save_page_to_file` | `href`, `save_to` | 以 headless Chrome 將 JS 渲染頁面存為本地檔案 |
| `run_command` | `command` | 在 sandbox 中執行白名單 shell 指令（300 秒 timeout） |
| `add_task` | `at`, `script`, `channel_id` | 排程一次性任務；完成時結果張貼至 Discord channel |
| `list_tasks` | — | 列出所有待執行的一次性任務 |
| `remove_task` | `index` | 取消並移除一次性任務 |
| `add_cron` | `cron_expr`, `script`, `channel_id` | 註冊 cron 任務；每次執行後結果張貼至 Discord channel |
| `list_crons` | — | 列出所有已註冊 cron 任務 |
| `remove_cron` | `index` | 以 index 移除 cron 任務 |
| `skill_git_commit` | `message` | 以給定訊息 commit skill repo 當前變更 |
| `skill_git_log` | `limit` | 顯示 skill repo 最近 commit |
| `skill_git_rollback` | `commit` | 將 skill repo 回復到指定 commit hash |
| `list_tools` | — | 列出所有當前可用工具，包含動態 API 擴充 |
| `calculate` | `expression` | 評估數學運算式（sqrt、abs、pow、ceil、floor、sin、cos、tan、log） |
| `invoke_external_agent` | `provider`, `task`, `readonly?` | 將整個任務委派至具名外部 agent（`copilot` / `claude` / `codex`） |
| `cross_review_with_external_agents` | `input`, `result` | 將結果平行送至所有宣告的外部 agent 並合併回饋；無外部 agent 時 fallback 到 `review_result` |
| `review_result` | `input`, `result` | 以優先序最高的可用模型做內部完整性覆核（claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet） |
| `invoke_subagent` | `task`, `model?`, `system_prompt?`, `exclude_tools?` | In-process 子 agent 委派，獨立暫時 session；子 agent 強制排除 `invoke_subagent` 以避免無限巢狀 |

## REST API

啟動統一應用後，REST API 會監聽 `PORT`（預設 `17989`）：

```bash
agen
# 或：make app
```

### Endpoint

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/v1/send` | 執行 agent 並回傳回應（SSE 或 JSON） |
| `GET` | `/v1/tools` | 列出所有已註冊工具 |
| `POST` | `/v1/tool/:name` | 直接呼叫單一工具 |
| `GET` | `/v1/key` | 從 OS Keychain 讀取憑證 |
| `POST` | `/v1/key` | 將憑證寫入 OS Keychain |

### POST /v1/send

執行完整的 agent 執行迴圈。設 `"sse": true` 以 Server-Sent Events 串流接收 token chunks。

**請求：**
```json
{ "content": "summarize today's news", "sse": false }
```

使用選用的 `model` 欄位繞過自動 agent 選擇，直接路由到特定模型（key 格式：`provider@model-name`）：

```json
{ "content": "summarize today's news", "sse": false, "model": "claude@claude-opus-4-6" }
```

使用 `exclude_tools` 僅在此請求中屏蔽特定工具：

```json
{ "content": "summarize today's news", "sse": false, "exclude_tools": ["run_command", "write_file"] }
```

**回應（非 SSE）：**
```json
{ "text": "..." }
```

**回應（SSE）：** `Content-Type: text/event-stream` — 每個 `data:` 行為一個 token chunk；agent 完成時 stream 關閉。

### GET /v1/tools

回傳所有已註冊工具（內建、API 擴充與 script 工具）。

### POST /v1/tool/:name

以 name 直接呼叫單一工具。請求 body 直接作為工具參數傳入。

**請求：**
```json
{ "query": "Bitcoin price", "time_range": "1d" }
```

### GET /v1/key · POST /v1/key

讀取或寫入 OS Keychain 中的憑證項目。Script 工具應透過此 endpoint 存取 keychain，而非直接呼叫。

### 從 script 工具呼叫 API

排程執行中的 script 工具可透過 `localhost` 呼叫 API：

```python
import json, urllib.request, os

BASE = f"http://localhost:{os.environ.get('PORT', '17989')}"

def call_tool(name, args):
    payload = json.dumps(args).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/tool/{name}",
        data=payload, headers={"Content-Type": "application/json"}, method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("result", "")
```

## Sandbox 隔離

所有透過 `run_command` 與 scheduler 腳本執行的指令都在 OS 原生 sandbox 中執行：

| 功能 | Linux（bwrap） | macOS（sandbox-exec） |
|------|----------------|----------------------|
| 檔案系統 | 唯讀 root、可寫 `$HOME` | 預設拒絕、`file-read*` 允許、`file-write*` 限於 `$HOME` |
| 敏感路徑拒絕 | `--tmpfs` / `--ro-bind /dev/null` 套用在敏感路徑 | Seatbelt `deny file-read*` / `deny file-write*` |
| 命名空間隔離 | `--unshare-user/pid/ipc/uts/cgroup`（逐項探測） | 不適用 |
| Session 隔離 | `--new-session` | 不適用 |
| 網路 | 允許（`--share-net`） | 允許（`allow network*`） |
| 防孤兒 | `--die-with-parent` | 不適用 |
| 路徑驗證 | `filepath.EvalSymlinks` → 超出 `$HOME` 即拒絕 | 同 |
| 自動安裝 | 啟動時偵測，缺少時透過套件管理員自動安裝 | 內建，無需安裝 |

## Agent 介面

```go
type Agent interface {
    Name() string
    MaxInputTokens() int
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` 處理單次 LLM API 呼叫。`Execute` 管理完整的 skill 執行迴圈，最多 128 次工具呼叫迭代，達上限時自動觸發摘要。`MaxInputTokens` 回傳模型的最大輸入 token 數，用於 session 層 token 預算裁剪。

## Provider Registry

```go
func Default(provider string) string
func Get(provider, model string) ModelItem
func Models(provider string) map[string]ModelItem
func InputBytes(provider, model string) int
func OutputTokens(provider, model string) int
func SupportTemperature(provider, model string) bool
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
