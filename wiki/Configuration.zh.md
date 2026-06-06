# 設定檔

> [English](Configuration.md)

## 檔案結構

```
~/.config/agenvoy/
├── config.json                       主設定（active session、dispatcher_model、kuradb_enabled、t_enabled、d_enabled、compats[]）
├── usage.json                        Token 使用量追蹤
├── runtime.uid                       Server 模式 singleton lock（僅 daemon 寫）
├── mcp.json                          全域 MCP server
├── allow_skill                       全域 skill always-allow 清單（每行一個 name）
├── .telegram                         已授權 Telegram chat ID（每行一個，OTP 通過後寫入）
├── scheduler/
│   ├── tasks.json                    一次性排程任務
│   └── crons.json                    週期 cron 任務
├── download/                         對話收進來的附件 + 生成的圖片（agenvoy-img-<uuid>.png）
├── skills/scheduler/                 隔離的 scheduler skill 目錄（<short>-<hash8>/SKILL.md）
└── sessions/
    └── <sid>/
        ├── bot.md                    Agent persona（frontmatter + body）
        ├── status.json               Active task / state
        ├── action.log                工具呼叫稽核軌跡（1 MB rotate，目標 768 KB；外部 process 行會 prefix）
        ├── summary.meta.json         {last_message_time: YYYY-MM-DD HH:MM:SS} —— 增量 summary 游標
        ├── input_history             per-session TUI 輸入歷史
        └── mcp.json                  Session 範圍 MCP server

~/.config/kuradb/
└── endpoint                          純文字 URL（隨機 port），KuraDB spawn 時寫入，disable 時移除

<project-root>/.agenvoy/
└── allow_skill                       Project 範圍 skill always-allow 清單（與 global 在載入時聯集）
```

歷史、summary、error memory、config 旗標存在 ToriiDB（位於 `~/.config/agenvoy/.store/`，由 ToriiDB 管理，使用者不直接編輯）。

## 專案內設定

```
configs/
├── jsons/
│   ├── providors/                    Provider 目錄（注意：拼寫是慣例）
│   │   ├── claude.json
│   │   ├── openai.json
│   │   ├── codex.json
│   │   ├── gemini.json
│   │   ├── copilot.json
│   │   └── nvidia.json
│   ├── denied_map.json               沙箱拒絕路徑
│   ├── exclude_list.json             listing/walking 排除路徑
│   └── white_list.json               允許路徑
└── prompts/
    ├── system_prompt.md              主 system prompt 模板
    ├── skill_execution.md            Skill 執行紀律
    ├── summary_prompt.md             Summary 生成 prompt
    ├── summary_merge_prompt.md       Summary 合併 prompt
    ├── summary_context.md            Summary context 注入
    ├── discord_system_prompt.md      Discord 介面 system prompt
    └── telegram_system_prompt.md     Telegram 介面 system prompt
```

`compat` provider entry 在使用者透過 `agen model add` 新增後與靜態 catalog 並列。

## 環境變數

從 repo 根目錄 `.env` 載入，`cmd/app/main.go init()` 透過 `godotenv` 處理。

| 變數 | 必填 | 預設 | 說明 |
|---|---|---|---|
| `MAX_HISTORY_MESSAGES` | 否 | `16` | 每輪送 agent 的歷史訊息上限 |
| `MAX_TOOL_ITERATIONS` | 否 | `16` | 單次 request 的 tool 呼叫迴圈上限 |
| `MAX_SKILL_ITERATIONS` | 否 | `128` | skill 執行的 tool 呼叫迴圈上限 |
| `MAX_EMPTY_RESPONSES` | 否 | `8` | 連續空回應放棄前的容忍次數 |
| `MAX_SESSION_TASKS` | 否 | `3`（cap `10`） | per-session 併發上限，超過排隊 |
| `MAX_SUBAGENT_TIMEOUT_MIN` | 否 | `10`（cap `60`） | `invoke_subagent` 總上限（分鐘） |
| `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` | 否 | `10`（cap `60`） | 外部 CLI subprocess 上限（分鐘） |
| `AGENT_SEND_TIMEOUT_SECONDS` | 否 | `600` | Exec 層 ceiling，用 `context.WithTimeout` 包 provider 呼叫。主要對 codex SSE（10m client timeout）有意義；非 SSE provider 因 `Client.Timeout=5m` 一律先 fire |
| `OPENAI_API_KEY` | 否 | — | 啟用語意搜尋（`text-embedding-3-small`）與 KuraDB embeddings |

外部 CLI agent（`codex` / `gh` / `claude` / `gemini`）以 `exec.LookPath` 自動偵測；只要 binary 存在於 `PATH` 即啟用，不需設定 env flag。

整數變數會 clamp 至文件 cap；`≤ 0` 退回預設。

## bot.md 格式

```markdown
***
name: <session 顯示名稱>     # 用於 :name 路由與 invoke_subagent name 參數
***

<persona 內容，自由 markdown>
```

body 每輪渲染進 system prompt 的 `## Bot Persona` 區段。frontmatter `name` 未設時預設 = session id。

`agen session new <name>` 同時建立 session 目錄與 bot.md，frontmatter `name` = `<name>`。`agen session switch <name>` 用 bot.md frontmatter `name` 查找 session（**只看 frontmatter，不 fallback 到 sid**）。

## Permission Mode

啟用模式（`single-confirm` vs `always-allow`）由 entry 決定：

| 進入點 | 模式 |
|---|---|
| `agen cli` | `single-confirm`（`AllowAll=false`） |
| `agen run` | `always-allow`（`AllowAll=true`） |
| Discord / REST | `always-allow` |
| Telegram | `single-confirm`（`AllowAll=false`；confirm gate 走 Telegram inline-keyboard SendSelect） |
| Subagent | 繼承父 ctx |

模式渲染進 system prompt 的 `## Permission Mode` 區段。**沒有**全域 env var override。

## MCP 設定

兩層、session 覆蓋 global。完整 schema 與 `${VAR}` 展開行為見 [MCP 整合](MCP-Integration.zh.md)。

## Provider 設定

Provider 定義在 `configs/jsons/providors/`（拼寫是慣例）。憑證**絕不**寫進 JSON —— 全部存 OS keychain，service 名 `agenvoy`。

### Compat provider URL storage 分軌

`compat` provider URL 走 **two-storage** 模型：

| 內容 | 位置 | 為何 |
|---|---|---|
| URL（如 `http://host:8000/v1`） | `~/.config/agenvoy/config.json` `compats[].URL` | 非機密、使用者可編輯 |
| API key（`COMPAT_<NAME>_API_KEY`） | OS keychain | 機密 |

URL 慣例對齊 Zed：使用者填到 `/v1` 為止（例：`http://localhost:11434/v1`），`compat/send.go` 只 append `/chat/completions`。`compat.New` 透過 `session.GetCompatURL(instanceName)` 讀 URL —— **非** keychain。`COMPAT_<NAME>_URL` keychain key 已下線（歷史 bug：TUI 寫 config、runtime 讀 keychain，永遠 fallback localhost）。

## KuraDB

啟用狀態為 config.json 的 `kuradb_enabled: bool`。透過 TUI 的 `/kuradb` 切換（**無** CLI 子命令 —— install.sh + sudo 需要真正的 TTY）。生命週期詳見 [KuraDB RAG](KuraDB-RAG.zh.md)。

| Key | 位置 |
|---|---|
| `kuradb_enabled` | `config.json` |
| `OPENAI_API_KEY` | keychain（`agenvoy` service） —— 與語意搜尋共用 |
| Endpoint URL（runtime） | `~/.config/kuradb/endpoint`（純文字，spawn 時隨機 port） |
| Binary | `/usr/local/bin/kura`（install.sh 內 hardcoded） |

## Telegram / Discord 啟用

| Key | 位置 |
|---|---|
| `telegram_enabled` / `discord_enabled` | `config.json` |
| `TELEGRAM_TOKEN` / `DISCORD_TOKEN` | keychain（`agenvoy` service） |
| 已授權 chat ID | `~/.config/agenvoy/.telegram`（一行一個 chat ID，6 碼 OTP 驗證成功後寫入） |
| 已授權 Discord channel | 透過 guild mention + per-server `d_allowed` 設定 |

## 哪些東西**刻意**不放在這裡

幾個刻意的非位置：

- **Provider API key** —— 不放 `config.json`，永遠在 keychain
- **MCP 憑證** —— `mcp.json` 用 `${VAR}` placeholder，實際值放 env var（或經 shell init 從 keychain 注入）
- **`store_secret` 取得的密鑰** —— 只進 keychain；不進 LLM context、history、action.log 或 tool args
- **Session 歷史** —— 在 ToriiDB，不在 per-session JSON 檔（ToriiDB v0.5.0 遷移後）
- **工具呼叫結果** —— 只 cache 在記憶體，不跨重啟持久化（除了透過 error_memory 與 conversation_history）

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
