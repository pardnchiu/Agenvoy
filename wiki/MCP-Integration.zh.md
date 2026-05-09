# MCP 整合

> [English](https://github.com/agenvoy/Agenvoy/wiki/MCP-Integration)

MCP（Model Context Protocol）client 讓 Agenvoy agent 能呼叫任何 MCP server 暴露的工具。

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
