# {repo} - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.20 或更高版本
- 至少一組 AI agent 憑證（GitHub Copilot 訂閱或 API key）

## 安裝

### 從原始碼建置

```bash
git clone https://github.com/{owner}/{repo}.git
cd {repo}
go build -o {repo} cmd/cli/main.go
```

### 使用 go install

```bash
go install github.com/{owner}/{repo}/cmd/cli@latest
```

## 設定

### 環境變數

| 變數 | 必要 | 說明 |
|------|------|------|
| `OPENAI_API_KEY` | 否 | OpenAI API 金鑰 |
| `ANTHROPIC_API_KEY` | 否 | Anthropic API 金鑰 |

複製 `.env.example` 並填入對應值：

```bash
cp .env.example .env
```

## 使用方式

### 基礎用法

列出所有可用的 Skill：

```bash
./{repo} list
```

### 執行 Skill

```bash
# 互動模式（每次 tool call 前確認）
./{repo} run commit-generate "generate commit message"

# 自動模式（跳過確認）
./{repo} run readme-generate "generate readme" --allow
```

## 命令列參考

### 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| `list` | `./{repo} list` | 列出所有已安裝的 Skill |
| `run` | `./{repo} run <skill> <input> [--allow]` | 執行指定的 Skill |

### 旗標

| 旗標 | 說明 |
|------|------|
| `--allow` | 跳過互動式確認提示 |

### 支援的 Agent

| Agent | 認證方式 | 預設模型 | 環境變數 |
|-------|----------|----------|----------|
| GitHub Copilot | Device code 登入 | `gpt-4.1` | - |
| OpenAI | API Key | `gpt-5-nano` | `OPENAI_API_KEY` |
| Claude | API Key | `claude-sonnet-4-5` | `ANTHROPIC_API_KEY` |

### 內建工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `read_file` | `path` | 讀取指定路徑的檔案內容 |
| `list_files` | `path`, `recursive` | 列出目錄內容 |
| `write_file` | `path`, `content` | 寫入或建立檔案 |
| `search_content` | `pattern`, `file_pattern` | 使用 regex 搜尋檔案內容 |
| `run_command` | `command` | 執行白名單內的 shell 指令 |

***

©️ {year} [{author_name}]({author_url})
