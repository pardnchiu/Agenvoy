# agenvoy - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.1 或更高版本
- 至少一組 AI agent 憑證（選擇一個或多個）：
  - GitHub Copilot 訂閱（互動式 Device Code 登入）
  - `OPENAI_API_KEY`（OpenAI）
  - `ANTHROPIC_API_KEY`（Claude）
  - `GEMINI_API_KEY`（Gemini）
  - `NVIDIA_API_KEY`（NVIDIA NIM）
  - 本地 Ollama 或任何 OpenAI 相容服務（compat provider，無需 API key）
- Chrome 瀏覽器（`fetch_page` 工具使用 go-rod，首次使用時自動下載）
- Discord Bot 模式需要：Discord Bot Token，以及選填的 Guild ID

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

### 從原始碼建置（Discord Bot Server）

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -o agenvoy-server ./cmd/server
```

### 作為函式庫使用

```bash
go get github.com/pardnchiu/agenvoy
```

## 設定

### 新增 Provider（互動式）

執行 `add` 指令以互動方式註冊 provider。憑證存放於系統 Keychain，無需手動設定環境變數。

```bash
agenvoy add
```

提示列出所有支援的 provider：

```
? Select provider to add:
  GitHub Copilot
  OpenAI
  Claude
  Gemini
  Nvidia
  Compat
```

- **GitHub Copilot**：開啟 Device Code 瀏覽器登入，再提示輸入模型名稱
- **API key provider**（OpenAI / Claude / Gemini / NVIDIA）：提示輸入 API key（遮罩輸入），存入系統 Keychain
- **Compat**：提示輸入 provider 名稱、端點 URL（預設：`http://localhost:11434`）、選填 API key 與模型名稱

### 憑證查找順序

每個 API key 依序從以下位置查找：
1. 系統 Keychain（macOS Keychain / Linux `secret-tool`）
2. 相同名稱的環境變數
3. `~/.config/agenvoy/.secrets`（其他平台的檔案備援）

### Agent 設定檔

在 `~/.config/agenvoy/config.json` 或 `./.config/agenvoy/config.json` 建立 agent 清單：

```json
{
  "default_model": "claude@claude-sonnet-4-5",
  "models": [
    {
      "name": "claude@claude-sonnet-4-5",
      "description": "高品質任務、文件生成、程式碼分析"
    },
    {
      "name": "openai@gpt-5-mini",
      "description": "一般查詢、快速回應"
    },
    {
      "name": "compat[ollama]@qwen3:8b",
      "description": "本地任務、離線使用"
    }
  ]
}
```

### Skill 檔案

Skill 是啟動時從 9 個標準路徑掃描的 Markdown 檔案：

| 優先順序 | 路徑 |
|----------|------|
| 1 | `./skills/` |
| 2 | `./.claude/skills/` |
| 3 | `~/.skills/` |
| 4 | `~/.claude/skills/` |
| 5 | `~/.config/agenvoy/skills/` |
| 6–9 | XDG / 系統層路徑 |

### Discord Bot 環境變數

複製 `.env.example` 並填入值：

```bash
cp .env.example .env
```

| 變數 | 必要 | 說明 |
|------|------|------|
| `DISCORD_TOKEN` | 是 | Discord bot token |
| `DISCORD_GUILD_ID` | 否 | 指定 Guild ID 可讓 slash command 即時生效（開發用）。留空則為全域註冊（最多需等待 1 小時）。 |

## 使用方式

### CLI — 基礎用法

```bash
agenvoy run 台積電目前股價是多少？
```

### CLI — 附加圖片

```bash
agenvoy run 描述這張圖表 --image ./chart.png
agenvoy run-allow 比較這兩張截圖 --image /tmp/before.png --image /tmp/after.png
```

### CLI — 附加檔案

```bash
agenvoy run 分析這份日誌 --file ./app.log
agenvoy run-allow 比較兩份設定檔 --file ./config.a.yaml --file ./config.b.yaml
```

### CLI — 自動模式

```bash
agenvoy run-allow 為目前的變更生成 commit message
```

### Discord Bot Server

```bash
# 複製並填入環境變數
cp .env.example .env

# 啟動 server
./agenvoy-server
```

Bot 回應以下觸發方式：
- **私訊**：任何訊息均觸發 Agentic 迴圈
- **頻道訊息**：只有 @mention bot 時才觸發

### 函式庫 — 嵌入執行引擎

```go
package main

import (
    "context"
    "fmt"

    "github.com/pardnchiu/agenvoy/internal/agents/exec"
    "github.com/pardnchiu/agenvoy/internal/agents/provider/claude"
    "github.com/pardnchiu/agenvoy/internal/agents/provider/openai"
    atypes "github.com/pardnchiu/agenvoy/internal/agents/types"
    "github.com/pardnchiu/agenvoy/internal/skill"
)

func main() {
    ctx := context.Background()

    claudeAgent, err := claude.New("claude@claude-sonnet-4-5")
    if err != nil {
        panic(err)
    }
    oaiAgent, err := openai.New("openai@gpt-5-mini")
    if err != nil {
        panic(err)
    }

    registry := atypes.AgentRegistry{
        Registry: map[string]atypes.Agent{
            "claude@claude-sonnet-4-5": claudeAgent,
            "openai@gpt-5-mini":        oaiAgent,
        },
        Entries: []atypes.AgentEntry{
            {Name: "claude@claude-sonnet-4-5", Description: "高品質任務"},
            {Name: "openai@gpt-5-mini", Description: "一般查詢"},
        },
        Fallback: claudeAgent,
    }

    selectorBot, _ := openai.New("openai@gpt-5-mini")
    scanner := skill.NewScanner()
    events := make(chan atypes.Event, 16)

    go func() {
        defer close(events)
        if err := exec.Run(ctx, selectorBot, registry, scanner, "查詢台積電股價", nil, nil, events, true); err != nil {
            fmt.Println("錯誤:", err)
        }
    }()

    for ev := range events {
        switch ev.Type {
        case atypes.EventText:
            fmt.Println(ev.Text)
        case atypes.EventDone:
            fmt.Println("完成")
        }
    }
}
```

## 命令列參考

### 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| `add` | `agenvoy add` | 互動式註冊 provider 並將憑證存入系統 Keychain |
| `remove` | `agenvoy remove` | 互動式移除已設定的 provider |
| `list` | `agenvoy list` | 列出已設定的模型 |
| `list skills` | `agenvoy list skills` | 列出所有已發現的 Skill |
| `run` | `agenvoy run <input...> [--image <path>]... [--file <path>]...` | 執行任務（互動模式，每次工具呼叫前確認） |
| `run-allow` | `agenvoy run-allow <input...> [--image <path>]... [--file <path>]...` | 執行任務（自動模式，跳過所有確認） |

### 支援的 Agent Provider

| Provider | 認證方式 | 預設模型 | 環境變數 | 圖片輸入 |
|----------|----------|----------|----------|----------|
| `copilot` | Device Code 互動登入 | `gpt-4.1` | — | ✓ |
| `openai` | API Key | `gpt-5-mini` | `OPENAI_API_KEY` | ✓ |
| `claude` | API Key | `claude-sonnet-4-5` | `ANTHROPIC_API_KEY` | ✓ |
| `gemini` | API Key | `gemini-2.5-pro` | `GEMINI_API_KEY` | ✓ |
| `nvidia` | API Key | `openai/gpt-oss-120b` | `NVIDIA_API_KEY` | ✗ |
| `compat` | 選填 API Key | 任意 | `COMPAT_{NAME}_API_KEY` | 依後端 |

模型格式：`{provider}@{model-name}`，例如 `claude@claude-opus-4-6`。<br>
Compat 格式：`compat[{name}]@{model}`，例如 `compat[ollama]@qwen3:8b`。

### 內建工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `read_file` | `path` | 讀取指定路徑的檔案內容 |
| `list_files` | `path`, `recursive` | 列出目錄內容 |
| `glob_files` | `pattern` | 以 glob 模式搜尋檔案（例如 `**/*.go`） |
| `write_file` | `path`, `content` | 寫入或建立檔案 |
| `patch_edit` | `path`, `old`, `new` | 精確字串替換（比 write_file 更安全） |
| `search_content` | `pattern`, `file_pattern` | 以 regex 搜尋檔案內容 |
| `search_history` | `keyword`, `time_range` | 搜尋目前 session 歷史；支援 `1d`/`7d`/`1m`/`1y` 篩選 |
| `run_command` | `command` | 執行白名單內的 shell 指令 |
| `fetch_page` | `url` | 透過 Chrome 取得 JS 渲染後的頁面（支援 SPA） |
| `download_page` | `href`, `save_to` | 將頁面下載為可讀 Markdown 儲存至本地檔案 |
| `search_web` | `query`, `range` | DuckDuckGo 搜尋，回傳標題/URL/摘要 |
| `fetch_yahoo_finance` | `symbol`, `range` | 即時股價與K線資料 |
| `fetch_google_rss` | `keyword`, `time` | Google News RSS 搜尋 |
| `fetch_weather` | `city` | 目前天氣與預報（省略 city 則以目前 IP 定位） |
| `send_http_request` | `url`, `method`, `headers`, `body` | 通用 HTTP 請求 |
| `calculate` | `expression` | 精確數學運算（支援 `^`、`sqrt`、`abs` 等） |

### 允許的 Shell 指令

`run_command` 工具僅限以下指令：
`git`、`go`、`node`、`npm`、`yarn`、`pnpm`、`python`、`python3`、`pip`、`pip3`、`ls`、`cat`、`head`、`tail`、`pwd`、`mkdir`、`touch`、`cp`、`mv`、`rm`、`grep`、`sed`、`awk`、`sort`、`uniq`、`diff`、`cut`、`tr`、`wc`、`find`、`jq`、`echo`、`which`、`date`、`docker`、`podman`

### 安全限制

檔案工具與 `run_command` 均透過 `internal/tools/file/embed/denied.json` 執行路徑層存取控制，以下路徑永遠封鎖：

| 類別 | 封鎖內容 |
|------|---------|
| SSH | `.ssh/` 目錄、`id_rsa`、`authorized_keys`、`known_hosts` 等 |
| Shell 歷史 | `.bash_history`、`.zsh_history`、`.zhistory` |
| Shell 設定 | `.zshrc`、`.bashrc`、`.bash_profile`、`.zprofile`、`.zshenv` |
| 雲端憑證 | `.aws/`、`.gcloud/`、`.docker/`、`.gnupg/` |
| 私鑰 | `.pem`、`.key`、`.p12`、`.pfx`、`.cer`、`.crt`、`.der` |
| 密鑰檔 | `.env`、`.env.*`、`.netrc`、`.git-credentials` |

## API 參考

### Agent 介面

```go
type Agent interface {
    Send(ctx context.Context, messages []Message, toolDefs []tools.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` 執行單次 LLM API 呼叫。`Execute` 管理包含工具迭代、快取與 session 寫入的完整 Skill 執行迴圈。

### AgentRegistry

```go
type AgentRegistry struct {
    Registry map[string]Agent  // 以名稱索引的 Agent 實例
    Entries  []AgentEntry      // 供 Selector Bot 使用的 Agent 描述清單
    Fallback Agent             // 路由失敗時的預設 Agent
}
```

### exec.Run

```go
func Run(
    ctx      context.Context,
    bot      Agent,           // Selector Bot（輕量模型）
    registry AgentRegistry,   // 可用 Agent 清單
    scanner  *skill.Scanner,  // Skill 掃描器
    input    string,          // 使用者輸入
    images   []string,        // 圖片路徑（選填）
    files    []string,        // 檔案路徑（選填，內容嵌入 prompt）
    events   chan<- Event,    // 事件輸出 channel
    allowAll bool,            // true = 跳過所有工具確認
) error
```

### Event 類型

```go
const (
    EventSkillSelect  // Skill 匹配開始
    EventSkillResult  // Skill 已匹配（或 "none"）
    EventAgentSelect  // Agent 路由開始
    EventAgentResult  // Agent 已選擇（或 "fallback"）
    EventText         // Agent 文字輸出
    EventToolCall     // 即將呼叫工具
    EventToolConfirm  // 等待使用者確認（allowAll=false）
    EventToolSkipped  // 使用者跳過工具
    EventToolResult   // 工具執行結果
    EventDone         // 目前請求完成
)
```

### skill.NewScanner

```go
func NewScanner() *Scanner
```

建立並執行跨 9 個標準路徑的並行 Skill 掃描。發現重複 Skill 名稱時，優先採用第一個找到的。

### keychain.Get / keychain.Set

```go
func Get(key string) string        // 從系統 Keychain 讀取，備援至環境變數
func Set(key, value string) error  // 寫入系統 Keychain
```

### discord.New

```go
func New(
    plannerAgent  agentTypes.Agent,
    agentRegistry agentTypes.AgentRegistry,
    skillScanner  *skill.SkillScanner,
) (*discordTypes.DiscordBot, error)
```

建立並連線 Discord bot session、註冊 slash command，並回傳 bot handle。當 `DISCORD_TOKEN` 未設定時回傳 `nil, nil`。

### Discord 檔案附件（從 Agent 回覆）

在 Discord 模式下運行的 Agent 可在回覆文字中嵌入 marker 來傳送本地檔案：

```
[SEND_FILE:/絕對路徑/檔案.png]
```

支援多個附件，marker 會在傳送前從可見訊息文字中移除。

### APIDocumentData（自訂 API 設定結構）

```go
type APIDocumentData struct {
    Name        string                       // 工具名稱（自動加上 api_ 前綴）
    Description string                       // 工具描述（LLM 路由決策依據）
    Endpoint    APIDocumentEndpointData      // URL、Method、ContentType、Timeout
    Auth        *APIDocumentAuthData         // 認證（bearer/apikey/basic）
    Parameters  map[string]APIParameterData  // 參數定義（含 required、default）
    Response    APIDocumentResponseData      // 回應格式（json 或 text）
}
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
