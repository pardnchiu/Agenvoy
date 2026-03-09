# agenvoy - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.20 或更高版本
- 至少一組 AI Provider 憑證（GitHub Copilot 訂閱、或任一 API Key）
- Discord Bot Token（僅限 Server 模式）

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

### 從原始碼建置（Discord Bot）

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

### 環境變數（Discord Bot 專用）

| 變數 | 必要 | 說明 |
|------|------|------|
| `DISCORD_TOKEN` | 是 | Discord Bot Token |
| `DISCORD_GUILD_ID` | 否 | 設定後僅限特定 Guild 接收 Slash Command |

建立 `.env` 並填入對應值：

```bash
DISCORD_TOKEN=your_token_here
DISCORD_GUILD_ID=optional_guild_id
```

### 自訂 API 工具

在 `~/.config/agenvoy/apis/` 或 `./examples/apis/` 放置 JSON 設定檔即可新增自訂 API 工具，格式與 OpenAI Tool Schema 相容，支援請求範本與回應解析。

## 使用方式

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
| `write_file` | `path`, `content` | 寫入或建立檔案 |
| `list_files` | `path`, `recursive` | 列出目錄內容 |
| `glob_files` | `pattern` | Glob 模式比對（如 `**/*.go`） |
| `search_content` | `pattern`, `file_pattern` | Regex 搜尋檔案內容 |
| `patch_edit` | `path`, `old`, `new` | 字串替換編輯 |
| `search_history` | `query` | 查詢 Session 歷史記錄 |
| `remember_error` | `key`, `decision` | 儲存工具錯誤決策 |
| `search_errors` | `query` | 檢索錯誤知識庫 |
| `fetch_yahoo_finance` | `symbol`, `interval`, `range` | 股票數據 |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google 新聞 RSS |
| `send_http_request` | `method`, `url`, `headers`, `body` | 通用 HTTP 請求 |
| `fetch_weather` | `city`, `days`, `hourly_interval` | 天氣資訊 |
| `search_web` | `query`, `time_range` | DuckDuckGo 網頁搜尋 |
| `fetch_page` | `url` | JS 渲染頁面轉 Markdown（唯讀） |
| `download_page` | `url`, `path` | JS 渲染頁面儲存至檔案 |
| `run_command` | `command` | 執行白名單內的 Shell 指令 |
| `calculate` | `expression` | 數學運算（sqrt、sin、cos、pow 等） |

### Agent 介面

```go
type Agent interface {
    Name() string
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` 處理單次 LLM API 呼叫。`Execute` 管理完整的 Skill 執行迴圈，最多 128 次 Tool Call 迭代，達到上限時自動觸發摘要。

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
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
