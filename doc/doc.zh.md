# agenvoy - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.1 或更高版本
- 至少一組 AI Agent 憑證（以下擇一）：
  - GitHub Copilot 訂閱（Device Code 互動登入）
  - `OPENAI_API_KEY`（OpenAI）
  - `ANTHROPIC_API_KEY`（Claude）
  - `GEMINI_API_KEY`（Gemini）
  - `NVIDIA_API_KEY`（NVIDIA NIM）
  - 本地 Ollama 或其他 OpenAI 相容服務（compat provider，無需 API Key）
- Chrome 瀏覽器（`fetch_page` 工具使用 go-rod 驅動，首次執行會自動下載）

## 安裝

### 使用 go install

```bash
go install github.com/pardnchiu/agenvoy/cmd/cli@latest
```

### 從原始碼建置

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -o agenvoy ./cmd/cli
```

### 作為函式庫引用

```bash
go get github.com/pardnchiu/agenvoy
```

## 設定

### 新增 Provider（互動式）

執行 `add` 指令以互動方式註冊 Provider。憑證儲存於 OS Keychain，無需手動設定環境變數。

```bash
agenvoy add
```

提示選單列出所有支援的 Provider：

```
? Select provider to add:
  GitHub Copilot
  OpenAI
  Claude
  Gemini
  Nvidia
  Compat
```

- **GitHub Copilot**：開啟 Device Code 瀏覽器登入，完成後提示輸入模型名稱
- **API Key Provider**（OpenAI / Claude / Gemini / NVIDIA）：提示輸入 API Key（遮罩輸入），儲存至 OS Keychain
- **Compat**：提示輸入 Provider 名稱、端點 URL（預設 `http://localhost:11434`）、選填 API Key 與模型名稱

### 憑證查找順序

每個 API Key 的查找優先順序：
1. OS Keychain（macOS Keychain / Linux `secret-tool`）
2. 同名環境變數
3. `~/.config/agenvoy/.secrets`（其他平台的檔案型退路）

仍可使用環境變數替代 `agenvoy add`。

### Agent 設定檔

在 `~/.config/agenvoy/config.json` 或 `./.config/agenvoy/config.json` 建立 Agent 清單：

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
      "description": "一般查詢、快速回答"
    },
    {
      "name": "compat[ollama]@qwen3:8b",
      "description": "本地任務、離線使用"
    }
  ]
}
```

`default_model` 指定的 Agent 會排在首位成為 Fallback。

### Skill 檔案

在以下任一路徑建立 `{skill-name}/SKILL.md`：

```
./.claude/skills/
./.skills/
~/.claude/skills/           ← 推薦個人技能存放位置
~/.opencode/skills/
~/.openai/skills/
~/.codex/skills/
/mnt/skills/public
/mnt/skills/user
/mnt/skills/examples
```

SKILL.md 格式：

```markdown
# skill-name

Description: 一句話說明此 Skill 的用途（供 Selector Bot 判斷）

## 詳細指令內容
...
```

### 自訂 API 工具

在 `~/.config/agenvoy/apis/` 或 `./.config/agenvoy/apis/` 放置 JSON 設定檔：

```json
{
  "name": "my_api",
  "description": "呼叫我的自訂服務",
  "endpoint": {
    "url": "https://api.example.com/v1/data",
    "method": "POST",
    "content_type": "json",
    "timeout": 10
  },
  "auth": {
    "type": "bearer",
    "env": "MY_API_KEY"
  },
  "parameters": {
    "query": {
      "type": "string",
      "description": "查詢字串",
      "required": true
    }
  },
  "response": {
    "format": "json"
  }
}
```

掛載後工具名稱自動變為 `api_my_api`，AI 可直接呼叫。`auth.type` 支援 `bearer`、`apikey`、`basic`。

## 使用方式

### 新增 Provider

```bash
agenvoy add
```

互動式設定任意支援的 Provider，憑證儲存至 OS Keychain。

### 列出已設定的模型

```bash
agenvoy list
```

### 列出所有可用 Skill

```bash
agenvoy list skills
```

輸出範例：

```
Found 3 skill(s):

• commit-generate
  從 git diff 產生單句繁體中文 commit message
  Path: /Users/user/.claude/skills/commit-generate

• readme-generate
  從原始碼分析自動生成雙語 README
  Path: /Users/user/.claude/skills/readme-generate
```

### 執行任務（互動模式）

```bash
agenvoy run 查詢台積電今日股價
```

每次工具呼叫前會出現確認提示：

```
[*] Skill: fetch-finance
[*] claude@claude-sonnet-4-5
[*] Fetch Ticker — 2330.TW (1d)
? Run fetch_yahoo_finance? [Yes/Skip/Stop]
```

### 執行任務（自動模式）

```bash
agenvoy run-allow 生成 README
```

`run-allow` 跳過所有工具確認提示，完全自動執行。

### 多行輸入

多個單詞直接作為多個 arg 傳入並自動串接，無需引號包裹。支援兩種寫法：

Shell Line Continuation（自然多行）：

```bash
agenvoy run 修正 main.go 的 bug \
  錯誤訊息：index out of range \
  以下是相關程式碼片段
```

內嵌 `\n` 跳脫（單行等效寫法）：

```bash
agenvoy run 修正 main.go 的 bug\n錯誤訊息：index out of range
```

兩種寫法傳入 Agent 的內容完全相同。

### 作為函式庫使用

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

    // 初始化 Agent
    claudeAgent, err := claude.New("claude@claude-sonnet-4-5")
    if err != nil {
        panic(err)
    }
    oaiAgent, err := openai.New("openai@gpt-5-mini")
    if err != nil {
        panic(err)
    }

    // 建立 Agent Registry
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

    // Selector Bot（用輕量模型做路由）
    selectorBot, _ := openai.New("openai@gpt-5-mini")

    scanner := skill.NewScanner()
    events := make(chan atypes.Event, 16)

    go func() {
        defer close(events)
        if err := exec.Run(ctx, selectorBot, registry, scanner, "查詢台積電股價", nil, events, true); err != nil {
            fmt.Println("Error:", err)
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
| `add` | `agenvoy add` | 互動式設定 Provider，憑證儲存至 OS Keychain |
| `remove` | `agenvoy remove` | 互動式移除已設定的 Provider |
| `list` | `agenvoy list` | 列出已設定的模型 |
| `list skills` | `agenvoy list skills` | 列出所有已掃描的 Skill |
| `run` | `agenvoy run <input...> [--image <path>]... [--file <path>]...` | 執行任務（互動模式，每次 Tool Call 前確認） |
| `run-allow` | `agenvoy run-allow <input...> [--image <path>]... [--file <path>]...` | 執行任務（自動模式，跳過所有確認） |

### 圖片輸入

以 `--image <path>` 附加圖片，支援多個。該旗標會在傳給 Agent 前從輸入字串中提取：

```bash
agenvoy run 描述這張圖表 --image ./chart.png
agenvoy run-allow 比較這兩張截圖 --image /tmp/before.png --image /tmp/after.png
```

> **注意：** `nvidia` provider 不支援圖片輸入。

### 檔案輸入

以 `--file <path>` 將本機檔案內容嵌入 prompt，支援多個。檔案內容以 `---\npath: <filename>\n---\n<content>` 格式注入：

```bash
agenvoy run 分析這份 log --file ./app.log
agenvoy run-allow 比較兩份設定檔 --file ./config.a.yaml --file ./config.b.yaml
```

### 支援的 Agent Provider

| Provider | 認證方式 | 預設模型 | 環境變數 | 圖片輸入 |
|----------|----------|----------|----------|----------|
| `copilot` | Device Code 互動登入 | `gpt-4.1` | — | ✓ |
| `openai` | API Key | `gpt-5-mini` | `OPENAI_API_KEY` | ✓ |
| `claude` | API Key | `claude-sonnet-4-5` | `ANTHROPIC_API_KEY` | ✓ |
| `gemini` | API Key | `gemini-2.5-pro` | `GEMINI_API_KEY` | ✓ |
| `nvidia` | API Key | `openai/gpt-oss-120b` | `NVIDIA_API_KEY` | ✗ |
| `compat` | 選填 API Key | 任意 | `COMPAT_{NAME}_API_KEY` | 取決於後端 |

模型格式：`{provider}@{model-name}`，例如 `claude@claude-opus-4-6`。
Compat 格式：`compat[{name}]@{model}`，例如 `compat[ollama]@qwen3:8b`。

### 內建工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `read_file` | `path` | 讀取指定路徑的檔案內容 |
| `list_files` | `path` | 列出目錄中的檔案與子目錄 |
| `glob_files` | `pattern` | 以 Glob 模式搜尋檔案（如 `**/*.go`） |
| `write_file` | `path`, `content` | 寫入或建立檔案 |
| `patch_edit` | `path`, `old`, `new` | 精確字串替換（比 write_file 安全）|
| `search_content` | `pattern`, `path`, `file_pattern` | Regex 搜尋檔案內容 |
| `search_history` | `keyword`, `time_range` | 搜尋當前 Session 歷史，支援 `1d`/`7d`/`1m`/`1y` 時間過濾 |
| `run_command` | `command` | 執行白名單 Shell 指令 |
| `fetch_page` | `url` | Chrome 渲染後擷取頁面（支援 SPA/動態頁面） |
| `search_web` | `query`, `range` | DuckDuckGo 搜尋，返回標題/網址/摘要 |
| `fetch_yahoo_finance` | `symbol`, `range` | 股票即時報價與 K 線資料 |
| `fetch_google_rss` | `keyword`, `time` | Google News RSS 新聞搜尋 |
| `fetch_weather` | `city` | 即時天氣與預報（可省略以取得目前位置） |
| `send_http_request` | `url`, `method`, `headers`, `body` | 通用 HTTP 請求 |
| `calculate` | `expression` | 精確數學運算（支援 `^`、`sqrt`、`abs` 等） |

### 允許的 Shell 指令

`run_command` 工具限制只能執行以下指令：
`git`, `go`, `node`, `npm`, `yarn`, `pnpm`, `python`, `python3`, `pip`, `pip3`, `ls`, `cat`, `head`, `tail`, `pwd`, `mkdir`, `touch`, `cp`, `mv`, `rm`, `grep`, `sed`, `awk`, `sort`, `uniq`, `diff`, `cut`, `tr`, `wc`, `find`, `jq`, `echo`, `which`, `date`, `docker`, `podman`

## API 參考

### Agent Interface

```go
type Agent interface {
    Send(ctx context.Context, messages []Message, toolDefs []tools.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` 發送單次 LLM API 請求。`Execute` 管理完整的 Skill 執行迴圈，包含工具迭代、快取與 Session 寫入。

### AgentRegistry

```go
type AgentRegistry struct {
    Registry map[string]Agent  // 依名稱索引的 Agent 實例
    Entries  []AgentEntry      // 供 Selector Bot 路由用的 Agent 描述清單
    Fallback Agent             // 路由失敗時使用的預設 Agent
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
    images   []string,        // 圖片路徑（選用）
    files    []string,        // 檔案路徑（選用，內容嵌入 prompt）
    events   chan<- Event,    // 事件輸出通道
    allowAll bool,            // true = 跳過所有工具確認
) error
```

### Event Types

```go
const (
    EventSkillSelect  // Skill 匹配開始
    EventSkillResult  // Skill 匹配完成（或 "none"）
    EventAgentSelect  // Agent 路由開始
    EventAgentResult  // Agent 選定（或 "fallback"）
    EventText         // Agent 輸出文字
    EventToolCall     // 工具即將被呼叫
    EventToolConfirm  // 等待使用者確認（allowAll=false 時觸發）
    EventToolSkipped  // 使用者跳過工具
    EventToolResult   // 工具執行結果
    EventDone         // 本次請求完成
)
```

### skill.NewScanner

```go
func NewScanner() *Scanner
```

建立並執行並發 Skill 掃描，掃描 9 個標準路徑。找到重複名稱的 Skill 時以先掃描到的為準。

### keychain.Get / keychain.Set

```go
func Get(key string) string        // 從 OS Keychain 讀取，退路為環境變數
func Set(key, value string) error  // 寫入 OS Keychain
```

### APIDocumentData（自訂 API 設定結構）

```go
type APIDocumentData struct {
    Name        string                       // 工具名稱（會自動加上 api_ 前綴）
    Description string                       // 工具說明（供 LLM 判斷使用時機）
    Endpoint    APIDocumentEndpointData      // URL、Method、ContentType、Timeout
    Auth        *APIDocumentAuthData         // 認證（bearer/apikey/basic）
    Parameters  map[string]APIParameterData  // 參數定義（含 required、default）
    Response    APIDocumentResponseData      // 回應格式（json 或 text）
}
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
