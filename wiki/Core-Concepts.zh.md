# 核心概念

> [English](Core-Concepts.md)

## Session

Session 是 Agenvoy 的核心單元，每個 session 對應獨立的對話上下文、記憶、agent persona 與工具配置。

儲存路徑：`~/.config/agenvoy/sessions/<sid>/`

| 檔案 | 說明 |
|---|---|
| `bot.md` | Agent persona 定義（YAML frontmatter + markdown body） |
| `status.json` | 當前執行狀態與 active task 陣列 |
| `action.log` | 工具呼叫稽核軌跡，1 MB 自動 rotate（截到 768 KB） |
| `mcp.json` | session 範圍的 MCP server 配置 |

歷史、summary、config 旗標都存在 ToriiDB（`DBSessionHist`、`DBSessionSummary`、`DBConfig`），不再是 per-session JSON。

### Session prefix 與 lifetime

| Prefix | 生命週期 |
|---|---|
| `cli-*` | 永久（`agen session new` / `make cli` 建立） |
| `http-*` | 永久（`POST /v1/send` 帶 `persist=true`） |
| `dc-*` | 永久（Discord 頻道） |
| `tg-*` | 永久（Telegram chat —— per-chat 共享，不分 user） |
| `temp-*` | 1h idle reap（`POST /v1/send` 預設） |
| `temp-sub-*` | 1h idle reap（subagent 預設） |

`runApp` 啟動時 `CleanupSessions()` 只清 `temp-*` 白名單；`cli-*`、`http-*`、`dc-*`、`tg-*` 永遠不會被自動清掉。

## bot.md — Agent Persona

每個 session 可以宣告自己的 persona：

```markdown
***
name: mobile-builder
***

You are an expert mobile application architect specializing in
SwiftUI, Jetpack Compose, and React Native...
```

frontmatter `name` 也是 lookup key（`GetSessionIDByName`）；body 在每輪 system prompt 的 `## Bot Persona` 區段渲染。`agen session config` 用 `$EDITOR` 開當前 session 的 bot.md。

## Agent 路由

三種方式決定任務交給哪個 agent：

**1. 自動選擇** —— dispatcher LLM 分析輸入後 `SelectAgent()` 挑最適 provider。

**2. `:name` 一次性 override**（CLI／TUI） —— 在輸入前加 `:session-name` 對指定 session 下指令、**不換主指標**：

```
:mobile-builder 幫我做 SwiftUI 登入畫面
```

`exec.Run` 解析順序：`:bot` → `MatchExternal`（`/claude` 等） → `MatchSkillCall`（`/skill-name`） → `Execute`。`:name` 在 `exec.Run`（CLI／TUI）與 Telegram runtime 解析（Telegram 命中即一次性覆寫、未命中則 strip prefix 並在 metadata 加 `備註` 行 fallback）；HTTP `POST /v1/send` 與 Discord 不解析此前綴。

**3. `invoke_subagent` 工具** —— agent 在執行中 in-process 呼叫另一個 agent（不走 HTTP），繼承父 ctx 的 `AllowAll`／`WorkDir`。強制排除集 `{invoke_subagent, invoke_external_agent, cross_review_with_external_agents, review_result}`；`ask_user` **不**排除 —— subagent 可透過共用 pending registry 向使用者提問。

## 執行迴圈

每次請求 `exec.Execute()` 主迴圈最多 **128 iteration**。每輪：

1. 組訊息：`SystemPrompts` + `OldHistories` + `UserInput` + `ToolHistories`
2. 呼叫所選 provider 的 `Agent.Send()`
3. 解析 response 中的 `tool_calls`
4. 透過 `toolCall.go` 派發（三段式併發，見下）
5. 結果寫回 `ToolHistories`
6. 沒有 `tool_calls` 或達 128 上限即停止

**無 inter-round delay** —— rate-limit 保護來自 provider round-trip 延遲、同錯 circuit breaker、tool 內部限流（例：`search_web` 2 s gap、`api_*` per-name 1 s gap）。

## 三段式工具併發

`toolCall.go` 把同一輪 tool calls 切成三段序列 pass，只有 Pass 2 才 fan out：

| Pass | 模式 | 內容 |
|---|---|---|
| 1 · pre-flight | 序列 | cache 命中檢查（`read_file` 跳過）、stub tool 短路、confirm gate、JSON schema 驗證 |
| 2 · 執行 | `IsConcurrent` 標記者併發；其餘序列 | `tools.Execute` |
| 3 · commit | 序列 | 落地 `sessionData.Tools`／`ToolHistories`、更新 cache、發 `EventToolResult`、處理 review tool |

Concurrent 標記：`read_file`、`list_files`、`glob_files`、`search_files`、`fetch_page`、`search_google_news`、`send_http_request`、`download_file`、`transcribe_media`、`calculate`、`invoke_subagent`、`search_chat_history`、`search_error_history`、`read_error`、`read_log`、`list_rag`、`search_rag`、`format_chatbot`、`list_chatbot`、`list_tools`、`list_schedule`。`search_web`／寫入類／`api_*`／MCP 一律序列。

## Pending Registry

`internal/runtime/pending.go` 是主 agent 與 in-process subagent 共用的前綴路由 confirm／ask listener registry。Producer（`toolCall` confirm、`ask_user` handler、`store_secret` handler）呼叫 `Ask(ctx, req)` 阻塞在 per-entry buffered=1 reply channel；各 runtime 透過 `pending.RegisterListener(prefix)` 註冊監聽器（TUI／CLI 用 `""` match all，Telegram daemon listener 用 `"tg-"`），並以 `PickNextFor(prefix)` 取對應條目。ctx cancel 即從 registry 移除，避免浪費使用者一次互動。

`pending.HasListener(sessionID)` 檢查該 session 是否有匹配 prefix 的 listener。這取代了舊版全域 `pending.Active atomic.Bool`，讓 Telegram、Discord、CLI 各自的 confirm 流程能並行不互阻塞。

## Circuit Breaker

`Agent.Send()` 連續三次回傳相同錯誤簽章（例如 HTTP 429 + 完全一樣的 request payload），主迴圈 abort，避免無限 retry。錯誤簽章不同會重置計數。

## Permission Mode

| 模式 | 行為 |
|---|---|
| `single-confirm` | 每個非 ReadOnly tool 呼叫前需使用者 confirm（`agen cli` 預設） |
| `always-allow` | 工具自動執行；LLM 被指示對七類真正不可逆操作須先 `ask_user` |

`always-allow` 下仍須顯式 `ask_user` 的七類：

1. 對非空目錄 `rm -rf`
2. `DROP TABLE` / `DROP DATABASE`
3. `git push --force` 至 `main`
4. 系統路徑 `chmod 777`
5. 覆蓋未 read 過的非空檔
6. cloud 資源 delete
7. 對系統 process `shutdown` / `kill -9`

這個 gate 由 system prompt 自律，不靠 Go 端 hardcoded 攔截 —— 新增類別只動 `configs/prompts/`。

## Per-session 併發

`MAX_SESSION_TASKS`（預設 `3`，hard cap `10`）限制單一 session 可同時執行的 `Execute()` 數。超過上限的 caller 在 `EnterConcurrent(sid)` 排隊，**不會**出現在 `status.json`，slot 釋放後才上線。

## 跨 turn workdir 重置

每個新 user message 都重新建 `Executor`，`data.WorkDir` 透過 `os.Getwd()` 重置為 process cwd —— `cd` 改動的 workdir **不跨 turn**。雙層護欄防止 LLM 從 history 推論到舊 workdir：

- **L1（system prompt）** —— `Work directory: {{.WorkPath}}` 並明示 prior `cd` 屬舊 turn 可能 stale
- **L2（per-message）** —— 所有 user message 包成 `---\n當前時間: ...\n工作目錄: <data.WorkDir>\n---\n<input>`；workDir 行是最強 anchor，覆蓋 history recency bias

TUI 透過 `stripUserMetaHeader` 視覺剝除 wrapper；LLM 仍收到原始字串。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
