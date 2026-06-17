# MCP 整合

> [English](MCP-Integration.md)

Agenvoy 同時是 MCP **client**（連接外部 MCP server）和 MCP **server**（將沙箱工具層暴露給外部 agent）。

## MCP Server — 外部 agent 的通用沙箱工具層

透過 stdio pipe 啟動時，`agen` 以 MCP server 模式運行。任何 MCP 相容 agent（Claude Code、Codex、OpenCode 等）都能連接並使用 Agenvoy 的工具——包括即時建立新工具。

### 外部 agent 獲得的能力

- **沙箱執行** — 所有 script tool 在 OS 原生沙箱（macOS `sandbox-exec` / Linux `bwrap`）內執行，隔離 `~/.ssh`、`~/.aws`、`.env`、`*.pem` 等敏感路徑
- **自動建工具** — 沒有現成 tool 時，agent 呼叫 `script_tool_generate_guide` 取得 contract，接著 `write_tool` → `test_tool` 建立新的 Python script tool。工具持久化，跨 session 可重用
- **共用工具庫** — 任何 agent（Agenvoy 內部、Claude Code、Codex 等）建立的工具都存在 `~/.config/agenvoy/tools/script/`，所有連接的 agent 共享。建一次，到處用
- **即時資料存取** — `api_public_api_list` 索引免費公開 API；agent 挑選後包成 script tool，用真實資料回答而非訓練知識猜測

### 快速設定

TUI：`/mcp install` → 選擇 agent

各 agent 手動設定：

**Claude Code** — `~/.claude.json`
```json
{ "mcpServers": { "agenvoy": { "command": "agen" } } }
```

**Codex** — `~/.codex/config.toml`
```toml
[mcp_servers.agenvoy]
command = "agen"
```

**OpenCode** — `~/.config/opencode/opencode.jsonc`
```json
{ "mcp": { "agenvoy": { "type": "local", "command": ["agen"] } } }
```

### 通用 MCP client 接入

未列出的 MCP client 只需提供：

- **Transport**：stdio（JSON-RPC over stdin/stdout）
- **Command**：`agen`
- **Args**：無
- **前置條件**：`agen` binary 在 `$PATH`（`curl -fsSL https://cloud.agenvoy.com/install.sh | bash`）

Server 使用 [MCP protocol version `2024-11-05`](https://spec.modelcontextprotocol.io/specification/2024-11-05/)，支援 `tools/list`（含 `listChanged` 推播）與 `tools/call`。無需驗證——server 以當前使用者身份在本地執行。

各 MCP client 的通用設定模式：

```json
{
  "<servers_key>": {
    "agenvoy": {
      "command": "agen"
    }
  }
}
```

`<servers_key>` 因 client 而異（`mcpServers`、`mcp_servers`、`mcp` 等）。部分 client 需明確指定 `"type": "stdio"` 或 `"type": "local"`。請參閱各 client 文件。

### 暴露的工具

| Tool | 用途 |
|---|---|
| `script_*` / `api_*` / `ext_*` | 使用者建立的工具與擴充工具（從磁碟自動探索） |
| `write_tool` | 寫入 tool.json 或 script.py 到 script tool 目錄 |
| `test_tool` | 在沙箱內跑 script tool 驗證 |
| `patch_tool` | 字串替換修正 tool 檔案 |
| `remove_tool` | 移至 trash |
| `list_tools` | 列出 server 暴露的所有 tool |
| `script_tool_generate_guide` | 回傳 Script Tool Contract（命名、模板、執行流程、checklist） |
| `api_public_api_list` | 依分類瀏覽免費公開 API 供建 tool 使用 |

工具 CRUD（`write_tool`、`test_tool`、`patch_tool`、`remove_tool`）與 Agenvoy 內部 runtime 共用——同一份 handler、同一份 schema，透過 `toolRegister` 橋接，無重複實作。

### 熱重載

Server 透過 `fsnotify` 監聽工具目錄。工具新增、修改或刪除時，server 自動重新掃描並推送 `notifications/tools/list_changed` —— client 不需重連即可刷新工具列表。

---

## MCP Client

MCP client 讓 Agenvoy agent 能呼叫任何 MCP server 暴露的工具。

## 配置結構

兩層 —— session 層覆蓋全域層：

```
~/.config/agenvoy/mcp.json                        ← 全域
~/.config/agenvoy/sessions/<sid>/mcp.json         ← session 範圍
```

### JSON 格式

```json
{
  "servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "${GITHUB_TOKEN}" }
    },
    "remote-api": {
      "url": "https://api.example.com/mcp",
      "headers": { "Authorization": "Bearer ${TOKEN}" }
    }
  }
}
```

`command` 與 `url` 互斥。`env`、`headers`、`args` 內的 `${VAR}` / `$VAR` 啟動時走 `os.Expand` 展開。

## Transport 類型

| 類型 | 設定欄位 | 適用場景 |
|---|---|---|
| stdio | `command` + `args` + `env` | 本地 CLI（`npx`、原生 binary） |
| HTTP + SSE | `url` + `headers` | 遠端服務、長駐 MCP server |

stdio 走 line-delimited JSON-RPC over stdin/stdout。HTTP transport 自動偵測 `Content-Type: text/event-stream`，否則回 plain JSON。

## CLI 管理

```bash
agen mcp list             # 列出所有已設定 MCP server（global + per-session）
agen mcp add              # promptui 互動新增
agen mcp remove           # 互動移除（含 scope 標籤）
```

`agen mcp add` 流程：

1. server 名稱
2. 類型 —— Local (stdio) / Remote (HTTP)
3. 類型對應欄位（command/args/env 或 url/headers）
4. Scope —— Global / 挑選 session

Scope 寫入鎖定**一個檔案** —— global 寫 `~/.config/agenvoy/mcp.json`、session 寫對應 `~/.config/agenvoy/sessions/<sid>/mcp.json`。不跨檔搬移。

## 工具命名

MCP 暴露的工具自動註冊為：

```
mcp__<server_name>__<tool_name>
```

範例：`mcp__github__create_issue`、`mcp__sqlite-notes__read_query`。

## 結果大小上限

每筆 MCP tool result 上限 **1 MiB**。超過即截斷並附 marker：

```
[mcp output truncated: <total> bytes total, <kept> kept; consider LIMIT / filter / pagination]
```

避免 OpenAI Responses API 對單一 tool output 10 MB 上限觸發 same-signature retry storm。SQLite `SELECT *` 大表會撞到 —— 加 `LIMIT` / `WHERE`。

## Confirm 行為

MCP tool 走最保守預設：

- `agen cli` —— 每個 MCP tool 呼叫逐個 confirm
- `agen run` —— 自動放行
- **無** per-server `read_only` 開關 —— agenvoy 不對第三方 server 擴張信任，因為其行為不可驗證（Slack MCP 可能默默發訊息、Filesystem MCP 可能默默寫檔）

要批次操作用 `agen run`；零星使用就接受 per-call confirm 成本。

## 生命週期

- **啟動**：`runApp` / `runAgent` 在 `buildAgentRegistry()` **之前**呼叫 `mcp.New(ctx, sid)` → `RegisterAll(ctx)` 並註冊 `defer Close()`
- **Per-server 失敗**：server 啟動失敗或 `ListTools` 失敗 → `slog.Warn` 跳過；不阻塞核心功能
- **啟動快照**：session ID 在首次解析鎖定；切 session 須重啟才會 reload server list

## 推薦 Server

零 auth、本地執行（不需 API key）：

| Server | 用途 |
|---|---|
| `mcp-server-sqlite` | 對本地 `.db` 跑 SQL |
| `@modelcontextprotocol/server-memory` | 持久化 knowledge graph |
| `@playwright/mcp` | 瀏覽器自動化（自下載 chromium） |
| `@modelcontextprotocol/server-postgres` | 連 local Postgres |
| `mcp-server-time` | 時區轉換 / 相對時間 |

避免註冊與內建工具能力重疊的 MCP server（例如 `filesystem`、`git`、`fetch`、`shell`）—— 重複只會撐大 LLM tool list。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
