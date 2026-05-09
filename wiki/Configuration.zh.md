# 設定檔

> [English](https://github.com/agenvoy/Agenvoy/wiki/Configuration)

## 檔案結構

```
~/.config/agenvoy/
├── config.json                       主設定（active session、預設值）
├── usage.json                        Token 使用量追蹤
├── runtime.uid                       Server 模式 singleton lock
├── mcp.json                          全域 MCP server
├── scheduler/
│   ├── tasks.json                    一次性排程任務
│   └── crons.json                    週期 cron 任務
└── sessions/
    └── <sid>/
        ├── bot.md                    Agent persona（frontmatter + body）
        ├── status.json               Active task / state
        ├── action.log                工具呼叫稽核軌跡（1 MB rotate，目標 768 KB）
        └── mcp.json                  Session 範圍 MCP server
```

歷史、summary、config 旗標存在 ToriiDB（位於 `~/.config/agenvoy/.store/`，由 ToriiDB 管理，使用者不直接編輯）。

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
    └── discord_system_prompt.md      Discord 介面 system prompt
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
| `DISCORD_TOKEN` | 否 | — | Discord Bot token；缺值即關閉 Discord 介面 |
| `DISCORD_GUILD_ID` | 否 | — | beta 模式立即註冊 slash command 的 guild id |
| `EXTERNAL_COPILOT` / `EXTERNAL_CLAUDE` / `EXTERNAL_CODEX` / `EXTERNAL_GEMINI` | 否 | — | 自訂外部 CLI binary 路徑 |
| `OPENAI_API_KEY` | 否 | — | 啟用語意搜尋（`text-embedding-3-small`） |

整數變數會 clamp 至文件 cap；`≤ 0` 退回預設。

## bot.md 格式

```markdown
---
name: <session 顯示名稱>     # 用於 :name 路由與 invoke_subagent name 參數
---

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
| Subagent | 繼承父 ctx |

模式渲染進 system prompt 的 `## Permission Mode` 區段。**沒有**全域 env var override。

## MCP 設定

兩層、session 覆蓋 global。完整 schema 與 `${VAR}` 展開行為見 [MCP 整合](https://github.com/agenvoy/Agenvoy/wiki/MCP-整合)。

## Provider 設定

Provider 定義在 `configs/jsons/providors/`（拼寫是慣例）。憑證**絕不**寫進 JSON —— 全部存 OS keychain，service 名 `agenvoy`。

## 哪些東西**刻意**不放在這裡

幾個刻意的非位置：

- **Provider API key** —— 不放 `config.json`，永遠在 keychain
- **MCP 憑證** —— `mcp.json` 用 `${VAR}` placeholder，實際值放 env var（或經 shell init 從 keychain 注入）
- **`store_secret` 取得的密鑰** —— 只進 keychain；不進 LLM context、history、action.log 或 tool args
- **Session 歷史** —— 在 ToriiDB，不在 per-session JSON 檔（ToriiDB v0.5.0 遷移後）
- **工具呼叫結果** —— 只 cache 在記憶體，不跨重啟持久化（除了透過 error_memory 與 conversation_history）
