# agenvoy - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

### 系統需求

- Go 1.20 或更高版本
- 至少一組 AI Provider 憑證（GitHub Copilot 訂閱、或任一 API Key）
- Discord Bot Token（僅限 Server 模式）

### 沙箱依賴

| 平台 | 依賴 | 說明 |
|------|------|------|
| Linux | `bubblewrap`（`bwrap`） | 啟動時自動偵測，未安裝則透過 `apt-get` / `dnf` / `yum` / `pacman` / `apk` 自動安裝 |
| macOS | `sandbox-exec` | 已內建於系統，無需額外安裝 |

### 瀏覽器依賴（選用）

- Chromium 或 Google Chrome — `fetch_page` 與 `download_page` 工具使用 headless 模式渲染頁面
- `go-rod` 會在首次使用時自動下載 Chromium（若系統未安裝）

### Go 相依套件

| 套件 | 用途 |
|------|------|
| `github.com/bwmarrin/discordgo` | Discord Bot API |
| `github.com/gin-gonic/gin` | REST API 伺服器（HTTP 路由） |
| `github.com/go-rod/rod` | Headless Chrome 瀏覽器自動化 |
| `github.com/go-shiori/go-readability` | HTML 內容擷取與清理 |
| `github.com/joho/godotenv` | `.env` 環境變數載入 |
| `github.com/manifoldco/promptui` | CLI 互動式選單 |
| `github.com/pardnchiu/go-scheduler` | Cron 表達式解析與排程 |
| `github.com/rivo/tview` | Terminal UI 框架 |
| `github.com/gdamore/tcell/v2` | Terminal cell 與事件函式庫 |
| `github.com/fsnotify/fsnotify` | 檔案系統事件監聽（TUI 檔案監視器） |
| `golang.org/x/image` | WebP 圖片解碼（Vision 輸入） |
| `golang.org/x/net` | HTML tokenizer 與網路工具 |
| `golang.org/x/term` | Terminal 狀態與原始模式控制 |

## 安裝

### 使用 go install

```bash
go install github.com/pardnchiu/agenvoy/cmd/cli@latest
```

### 從原始碼建置（CLI）

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -o agenvoy ./cmd/cli
```

### 從原始碼建置（統一進入點：TUI + Discord + REST API）

```bash
go build -o agenvoy-app ./cmd/app
```

### 從原始碼建置（僅 Discord Bot）

```bash
go build -o agenvoy-server ./cmd/server
```

## 設定

### 新增 Provider

執行互動式設定流程，從內嵌模型登錄檔選擇 Provider 與模型：

```bash
agenvoy add
```

支援的 Provider：

| Provider | 認證方式 | 預設模型 |
|----------|----------|----------|
| GitHub Copilot | OAuth Device Code Flow（自動刷新） | `gpt-4.1` |
| OpenAI | API Key（keychain） | `gpt-5-mini` |
| Claude | API Key（keychain） | `claude-sonnet-4-5` |
| Gemini | API Key（keychain） | `gemini-2.5-pro` |
| NVIDIA | API Key（keychain） | `openai/gpt-oss-120b` |
| Compat | 選填 API Key（keychain） | 使用者指定 |

### 環境變數

| 變數 | 必要 | 說明 |
|------|------|------|
| `DISCORD_TOKEN` | 是（Server 模式） | Discord Bot Token |
| `DISCORD_GUILD_ID` | 否 | 設定後僅限特定 Guild 接收 Slash Command |
| `PORT` | 否 | REST API 伺服器監聽埠（預設：`17989`） |
| `MAX_HISTORY_MESSAGES` | 否 | 傳送至 Agent 的最大歷史訊息數（預設：16） |
| `MAX_TOOL_ITERATIONS` | 否 | 每次請求的最大工具呼叫迭代次數（預設：16） |
| `MAX_SKILL_ITERATIONS` | 否 | Skill 執行中的最大工具呼叫迭代次數（預設：128） |
| `MAX_EMPTY_RESPONSES` | 否 | 連續空回應的最大次數，超過則中止（預設：8） |

建立 `.env` 並填入對應值：

```bash
cp .env.example .env
```

> 檔名含 `.example` 的檔案（如 `.env.example`）不受環境變數前綴封鎖規則限制，可安全讀取。

### API Extension

在 `~/.config/agenvoy/apis/` 放置 JSON 檔即可新增自訂 API 工具，每個檔案定義一個可呼叫的工具，啟動時自動載入：

```json
{
  "name": "my_tool",
  "description": "Agent 選擇工具時看到的說明",
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
      "description": "資源 ID",
      "required": true
    },
    "status": {
      "type": "string",
      "description": "依狀態篩選",
      "required": false,
      "default": "active",
      "enum": ["active", "inactive", "all"]
    }
  },
  "response": {
    "format": "json"
  }
}
```

| 欄位 | 必要 | 說明 |
|------|------|------|
| `name` | 是 | 向 Agent 登錄的 snake_case 工具名稱 |
| `description` | 是 | LLM 選擇工具時看到的用途描述 |
| `endpoint.url` | 是 | 目標 URL；`{param}` 佔位符在呼叫時替換為實際值 |
| `endpoint.method` | 是 | HTTP 方法：`GET`、`POST`、`PUT`、`DELETE`、`PATCH` |
| `endpoint.content_type` | 否 | `json`（預設）或 `form` |
| `endpoint.headers` | 否 | 靜態 Header 鍵值對 |
| `endpoint.timeout` | 否 | 請求逾時秒數（預設：30） |
| `auth.type` | 否 | `bearer` 或 `apikey` |
| `auth.env` | 否 | 持有憑證的環境變數名稱 |
| `auth.header` | 否 | `apikey` 類型的 Header 名稱（預設：`X-API-Key`） |
| `parameters` | 是 | 參數定義的 flat 物件 |
| `response.format` | 否 | `json`（預設）或 `text` |

每個參數支援：`type`（`string` / `integer` / `number` / `boolean`）、`description`、`required`、`default`、`enum`。

#### 內嵌公開 API Extension

以下 API Extension 已內嵌並於啟動時自動載入：

| Extension | 分類 | 說明 |
|-----------|------|------|
| `nominatim` | 地理編碼 | OpenStreetMap 地理編碼與反向地理編碼 |
| `coingecko` | 金融 | 加密貨幣價格與市場數據 |
| `yahoo-finance-1/2` | 金融 | 股票報價與歷史數據 |
| `wikipedia` | 資料 | Wikipedia 文章搜尋與內容 |
| `world-bank` | 資料 | 世界銀行發展指標 |
| `usgs-earthquake` | 資料 | USGS 地震數據 |
| `themealdb` | 資料 | 食譜與餐點資料庫 |
| `hackernews` | 資料 | Hacker News 熱門文章與項目 |
| `rest-countries` | 資料 | 國家資訊與元數據 |
| `exchange-rate` | 金融 | 貨幣匯率 |
| `ip-api` | 網路 | IP 地理位置查詢 |
| `open-meteo` | 天氣 | 開源天氣預報 API |
| `youtube` | 媒體 | YouTube 影片 metadata（標題、描述、頻道、時長） |

### Script Tool Extension

在 `~/.config/agenvoy/script_tools/`（或 `<workdir>/.config/agenvoy/script_tools/`）放入包含 `tool.json` + `script.js`/`script.py` 的子目錄，執行器在啟動時自動掃描並以 `script_` 前綴登錄工具。

#### 內建 Extension 安裝腳本

本儲存庫附帶跨平台安裝腳本，可一行指令完成 Script Tool 部署：

```bash
# 安裝 Threads API 工具（發布文字/圖片/輪播、配額查詢、Token 刷新）
bash install_threads.sh

# 安裝 yt-dlp 工具（影片資訊、下載含檔名正規化）
bash install_youtube.sh
```

兩支腳本均自動偵測作業系統、驗證 Python 及相依套件，並將工具複製至 `~/.config/agenvoy/script_tools/`，下次啟動後即自動登錄。

| 內建工具 | 語言 | 說明 |
|---|---|---|
| `script_threads_get_quota` | Python | 查詢 Threads API 使用配額 |
| `script_threads_publish_text` | Python | 發布文字貼文（含 500 字元前置驗證） |
| `script_threads_publish_image` | Python | 發布圖片貼文附說明文字 |
| `script_threads_publish_carousel` | Python | 發布多圖輪播貼文 |
| `script_threads_refresh_token` | Python | 刷新 Threads 長效存取 Token |
| `script_yt_dlp_info` | JS / Python | 不下載直接擷取影片 metadata |
| `script_yt_dlp_downloader` | Python | 下載影片並 NFC 正規化檔名 |

Script tool 目錄結構：

```
~/.config/agenvoy/script_tools/
└── my-tool/
    ├── tool.json       # 工具描述檔
    └── script.py       # 或 script.js
```

`tool.json` 格式：

```json
{
  "name": "my_tool",
  "description": "Agent 選擇工具時看到的說明",
  "parameters": {
    "type": "object",
    "properties": {
      "input": {
        "type": "string",
        "description": "輸入值"
      }
    },
    "required": ["input"]
  }
}
```

Script I/O 契約 — 執行器將工具參數以 JSON 透過 stdin 傳入，從 stdout 讀取結果：

```python
#!/usr/bin/env python3
import json, sys

params = json.loads(sys.stdin.read() or "{}")
result = {"output": params.get("input", "").upper()}
print(json.dumps(result))
```

```js
const chunks = [];
process.stdin.on("data", d => chunks.push(d));
process.stdin.on("end", () => {
  const params = JSON.parse(Buffer.concat(chunks).toString() || "{}");
  console.log(JSON.stringify({ output: (params.input || "").toUpperCase() }));
});
```

使用 `script-tool-creator` Skill 自動產生新工具骨架：

```bash
agenvoy run-allow "建立一個可以查詢城市天氣的 script tool"
```

### Skill Extension

Skill Extension 是帶有 YAML Frontmatter 標頭的 Markdown 檔。啟動時 SyncSkills 會從 GitHub 儲存庫的 `extensions/skills` 下載本地尚不存在的 Skill 目錄，儲存至 `~/.config/agenvoy/skills/`。Agent 接著掃描所有 9 個標準路徑以建立可用 Skill 清單。

Skill 檔案格式（`SKILL.md`）：

```markdown
---
name: my-skill
description: 顯示給 Agent 選擇時的一行摘要
---

# My Skill

此 Skill 被選中時 Agent 遵循的指令...
```

掃描路徑（依優先順序）：

| 優先級 | 路徑 |
|--------|------|
| 1 | `~/.config/agenvoy/skills/`（從 GitHub 同步 + 使用者自訂） |
| 2–9 | XDG config 目錄、home 目錄與專案本地路徑 |

## REST API

啟動統一進入點後，REST API 監聽於 `PORT`（預設：`17989`）：

```bash
./agenvoy-app
# 或：go run ./cmd/app
```

### 端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| `POST` | `/v1/send` | 執行 Agent 並回傳回應（SSE 或 JSON） |
| `GET` | `/v1/tools` | 列出所有已登錄的工具 |
| `POST` | `/v1/tool/:name` | 直接呼叫單一工具 |
| `GET` | `/v1/key` | 從 OS Keychain 取得儲存的憑證 |
| `POST` | `/v1/key` | 儲存憑證至 OS Keychain |

### POST /v1/send

執行完整的 Agent 迭代迴圈。設定 `"sse": true` 以 Server-Sent Events 串流接收 token。

**請求：**
```json
{ "content": "幫我整理今天的新聞", "sse": false }
```

**回應（非 SSE）：**
```json
{ "text": "..." }
```

**回應（SSE）：** `Content-Type: text/event-stream`，每行 `data:` 為一個 token chunk；Agent 完成後串流關閉。

### GET /v1/tools

回傳所有已登錄的工具（內建、API Extension、Script Tool）。

**回應：**
```json
{
  "tools": [
    { "name": "search_web", "description": "...", "parameters": { ... } }
  ]
}
```

### POST /v1/tool/:name

依工具名稱直接呼叫工具，request body 直接作為工具參數傳入。

**請求：**
```json
{ "query": "Bitcoin 價格", "time_range": "1d" }
```

**回應：**
```json
{ "result": "..." }
```

### GET /v1/key · POST /v1/key

讀取或寫入 OS Keychain 中的憑證。Script Tool 應透過此端點存取憑證，而非直接操作 Keychain。

**POST 請求：**
```json
{ "service": "my-service", "key": "secret-value" }
```

**GET 請求：** `?service=my-service`

**GET 回應：**
```json
{ "key": "secret-value" }
```

### 在 Script Tool 中呼叫 API

排程任務內執行的腳本可直接透過 `localhost` 呼叫：

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

def send(prompt):
    payload = json.dumps({"content": prompt, "sse": False}).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/send",
        data=payload, headers={"Content-Type": "application/json"}, method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("text", "")
```

---

## 使用方式

### 使用 Make

於專案根目錄執行（需從原始碼 Clone）：

| Target | 實際指令 | 說明 |
|--------|---------|------|
| `make app` | `go run ./cmd/app/main.go` | 啟動統一進入點（TUI + Discord + REST API） |
| `make discord` | `go run ./cmd/server/main.go` | 啟動 Discord Bot Server（舊版） |
| `make add` | `go run ./cmd/cli/ add` | 互動式新增 Provider／模型 |
| `make remove` | `go run ./cmd/cli/ remove` | 移除已設定的 Provider |
| `make planner` | `go run ./cmd/cli/ planner` | 設定 Planner 模型 |
| `make list` | `go run ./cmd/cli/ list` | 列出已設定的模型 |
| `make skill-list` | `go run ./cmd/cli/ list skill` | 列出可用的 Skill |
| `make cli <input...>` | `go run ./cmd/cli/ run <input>` | 以確認模式執行 Agent |
| `make run <input...>` | `go run ./cmd/cli/ run-allow <input>` | 自動批准所有 Tool Call 並執行 Agent |

### 基礎用法

列出所有已設定的模型：

```bash
agenvoy list
```

列出所有可用的 Skill：

```bash
agenvoy list skills
```

以互動模式執行（每次 Tool Call 前確認）：

```bash
agenvoy run "幫我分析這個專案的架構"
```

### 進階用法

自動批准模式（跳過所有確認提示）：

```bash
agenvoy run-allow "生成並寫入 README 文件"
```

附加圖片輸入：

```bash
agenvoy run --image ./screenshot.png "這張圖在描述什麼？"
```

附加檔案輸入：

```bash
agenvoy run --file ./report.pdf "總結這份報告的重點"
```

移除 Provider：

```bash
agenvoy remove
```

## 命令列參考

### 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| `add` | `agenvoy add` | 互動式新增 AI Provider 設定 |
| `remove` | `agenvoy remove` | 移除已設定的 Provider |
| `planner` | `agenvoy planner` | 設定 Planner（路由器）模型 |
| `reasoning` | `agenvoy reasoning` | 設定 Provider 的推理層級（Reasoning Level） |
| `list` | `agenvoy list [skills]` | 列出已設定的模型或可用 Skill |
| `run` | `agenvoy run <input...> [flags]` | 以互動確認模式執行 Agentic 工作流 |
| `run-allow` | `agenvoy run-allow <input...> [flags]` | 自動批准所有 Tool Call |

### 旗標（run / run-allow）

| 旗標 | 說明 |
|------|------|
| `--image <path>` | 附加圖片輸入 |
| `--file <path>` | 附加檔案輸入 |

### 內建工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `read_file` | `path` | 讀取指定路徑的檔案內容 |
| `write_file` | `path`, `content` | 寫入或建立檔案（原子性寫入） |
| `list_files` | `path`, `recursive` | 列出目錄內容 |
| `glob_files` | `pattern` | Glob 模式比對（如 `**/*.go`） |
| `search_content` | `pattern`, `file_pattern` | Regex 搜尋檔案內容 |
| `patch_edit` | `path`, `old_string`, `new_string` | 第一個匹配項字串替換（比全檔覆寫更安全） |
| `search_history` | `keyword`, `time_range` | 查詢當前 Session 歷史記錄 |
| `get_tool_error` | `hash` | 透過 hash 取得失敗工具呼叫的完整錯誤詳情 |
| `remember_error` | `tool_name`, `keywords`, `symptom`, `action` | 儲存工具錯誤決策至知識庫 |
| `search_errors` | `keyword` | 檢索錯誤知識庫 |
| `analyze_youtube` | `url` | YouTube 影片 metadata（標題、描述、頻道、時長、觀看數） |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google 新聞 RSS（含去重） |
| `send_http_request` | `method`, `url`, `headers`, `body` | 通用 HTTP 請求 |
| `search_web` | `query`, `time_range` | 並行網頁搜尋（Google + DuckDuckGo） |
| `fetch_page` | `url` | 無頭 Chrome 渲染頁面轉 Markdown（唯讀） |
| `download_page` | `href`, `save_to` | JS 渲染頁面儲存至本地檔案 |
| `run_command` | `command` | 於沙箱中執行白名單內的 Shell 指令（300 秒逾時） |
| `write_script` | `name`, `content` | 在排程器目錄建立 `.py` 腳本 |
| `add_task` | `at`, `script`, `channel_id` | 設定一次性定時任務；執行結果傳送至指定 Discord 頻道 |
| `list_tasks` | — | 列出所有待執行的一次性任務 |
| `remove_task` | `index` | 依序號取消一次性任務（多個時須先列出） |
| `add_cron` | `cron_expr`, `script`, `channel_id` | 新增週期性 Cron 任務；每次執行結果傳送至指定 Discord 頻道 |
| `list_crons` | — | 列出所有已登錄的 Cron 任務 |
| `remove_cron` | `index` | 依序號移除 Cron 任務（多個時須先列出） |
| `skill_git_commit` | `message` | 以指定訊息提交 Skill 儲存庫的當前變更 |
| `skill_git_log` | `limit` | 顯示 Skill 儲存庫的近期提交歷史 |
| `skill_git_rollback` | `commit` | 將 Skill 儲存庫回滾至指定的 commit hash |
| `list_tools` | — | 列出所有可用工具，含動態載入的 API Extension |
| `calculate` | `expression` | 數學運算（sqrt、abs、pow、ceil、floor、sin、cos、tan、log） |

### 沙箱隔離

所有透過 `run_command` 執行的指令與排程器腳本均在作業系統原生沙箱中執行：

| 特性 | Linux（bwrap） | macOS（sandbox-exec） |
|------|---------------|----------------------|
| 檔案系統 | 唯讀根目錄，僅 `$HOME` 可寫 | 預設拒絕，允許 `file-read*`，`file-write*` 限縮至 `$HOME` |
| 敏感路徑封鎖 | `--tmpfs` 覆蓋敏感目錄、`--ro-bind /dev/null` 覆蓋敏感檔案 | Seatbelt `deny file-read*` / `deny file-write*` 規則 |
| Namespace 隔離 | `--unshare-user/pid/ipc/uts/cgroup`（逐一探測可用性） | 不支援 |
| Session 隔離 | `--new-session` | 不支援 |
| 網路 | 允許（`--share-net`） | 允許（`allow network*`） |
| 孤兒程序防護 | `--die-with-parent` | 不支援 |
| 路徑驗證 | `filepath.EvalSymlinks` → 超出 `$HOME` 則拒絕 | 相同 |
| 自動安裝 | 啟動時偵測，未安裝則自動透過套件管理器安裝 | 內建，無需安裝 |

### Token 用量追蹤

每次 LLM API 呼叫回傳 input/output token 數量，在單次執行 Session 內所有迭代中累計（含工具呼叫迴圈與最終摘要）。完成時顯示總量：

- **CLI**：`(耗時) [模型 | in:N out:N]`
- **Discord**：頁尾 `-# 模型 | in:N out:N`

各 Provider 的格式差異透明處理：Claude（`input_tokens`/`output_tokens`）、OpenAI 相容（`prompt_tokens`/`completion_tokens`）、Gemini（`promptTokenCount`/`candidatesTokenCount`）統一透過自訂 `UnmarshalJSON` 正規化為 `Usage` struct。

### 工具執行錯誤追蹤

任何工具呼叫失敗時，錯誤持久化至 Session 目錄的 `tool_errors/{hash}.json`，Agent 收到 `no data: {hash}` 作為結果。Agent 可呼叫 `get_tool_error` 帶入 8 位元 hex hash 取得完整錯誤資訊（tool 名稱、參數、錯誤訊息）。錯誤同時透過 `EventExecError` 事件即時通知：CLI 模式輸出至 stderr，Discord 模式附加於回覆頁尾。

### Agent 介面

```go
type Agent interface {
    Name() string
    MaxInputTokens() int
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` 處理單次 LLM API 呼叫。`Execute` 管理完整的 Skill 執行迴圈，最多 128 次 Tool Call 迭代，達到上限時自動觸發摘要。`MaxInputTokens` 回傳模型的最大輸入 token 數，用於 session 層級的 token-budget 裁剪。

### Provider Registry

```go
// 取得 Provider 的預設模型名稱
func Default(provider string) string

// 取得特定模型的 Context 限制與描述
func Get(provider, model string) ModelItem

// 列出 Provider 所有可用模型
func Models(provider string) map[string]ModelItem

// 計算最大輸入位元組數（token × 4，適用 UTF-8）
func InputBytes(provider, model string) int

// 取得最大輸出 Token 數
func OutputTokens(provider, model string) int

// 確認該模型是否支援 temperature 參數
func SupportTemperature(provider, model string) bool
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
