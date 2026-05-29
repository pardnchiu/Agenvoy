# 命令列參考

> [English](CLI-Reference.md)

## 頂層派發

`agen` 解析 `os.Args[1]` 並派發到下列 subcommand；無 subcommand 進 TUI（若 daemon 未跑會 fork-exec 一個）。

```bash
agen                                           # Attach TUI；daemon 未跑則 fork-exec
agen model   {add|remove|list|dispatcher|reasoning}
agen session {new|switch|config} [name]
agen mcp     {list|add|remove}
agen cli     <input...>                        # 單次，每 tool 需 confirm
agen run     <input...>                        # 單次，所有 tool 自動放行
agen stop                                      # 停止 daemon
agen update                                    # 下載最新版重新 build 安裝
```

### `agen model`

| Subcommand | 行為 |
|---|---|
| `add` | 互動式新增 provider／model（憑證寫入 keychain） |
| `remove`（alias `rm`） | 互動式移除 provider／model |
| `list` | 列出已註冊 model |
| `dispatcher` | 選 dispatcher model |
| `reasoning` | 設 dispatcher 推理層級：`low` / `medium` / `high` / `xhigh` |

### `agen session`

| Subcommand | 行為 |
|---|---|
| `new <name>` | 建 `cli-<uuid>` session、寫 `bot.md`（frontmatter `name=<name>`）、切主指標 |
| `switch [name]` | 切主指標；無 `name` 進互動 picker，當前 session 高亮（Enter 留原地） |
| `config [name]` | 用 `$EDITOR` 開目標 session 的 `bot.md`；無 `name` 進 picker |

### `agen mcp`

| Subcommand | 行為 |
|---|---|
| `list` | 列出所有 MCP server（global + per-session） |
| `add` | 互動式新增：名稱 → transport（Local stdio / Remote HTTP）→ 欄位 → scope（Global / pick session） |
| `remove` | 互動式移除（含 scope 標籤） |

### `agen stop`

對 daemon 發 SIGTERM（5 秒寬限後 SIGKILL）並清 `~/.config/agenvoy/runtime.uid`。daemon 不存在時印 `No daemon running.` 退出 0。

### `agen update`

Always-overwrite 升級至最新 release。從 `https://cloud.agenvoy.com/update.sh` 下載到 `/tmp/agenvoy-update-*.sh`、`bash` 執行、結束時 defer 移除 tmp 檔（SIGINT／SIGTERM 也會清）。內部 script 把最新 tag clone 到 `mktemp -d "${TMPDIR:-/tmp}/agenvoy-update.XXXXXX"`、跑 `make build`，最後印一個 summary box 提示下一步跑 `agen`。daemon 仍持舊 inode 跑——須手動 `agen stop` + 重 attach 才會載入新 binary。

## `make` 捷徑

```bash
make build                      # 編譯並安裝至 /usr/local/bin/agen
make app                        # 完整堆疊（TUI + Discord + Telegram + REST API）
make stop                       # 停止 daemon
make update                     # = agen update
make cli <input...>             # = agen cli <input...>
make run <input...>             # = agen run <input...>
make model   [add|remove|list|dispatcher|reasoning]
make session [new|switch|config] [name]
make mcp     [list|add|remove]
```

## TUI 快捷鍵

單一 bubbletea textarea（`internal/runtime/tui`）；slash command 開暫時 popup，結束自動回 prompt。

| 鍵 | 行為 |
|---|---|
| `Ctrl+S` | 送出 textarea 內容（Enter 改插換行；`Alt+Enter` 亦插換行） |
| `/` | 觸發 slash command 選單（popup picker — 上／下移動、Tab／Enter 補完到 textarea、Esc 關閉） |
| `Up` / `Down`（空 textarea 或單行時） | 走訪 input history（per-session `input_history` 檔） |
| `Esc` | 中斷目前 exec（若正在跑）或關閉當前 popup |
| `Ctrl+C` | 退出 TUI（daemon 仍在跑） |

TUI 自動 tail 當前 session 的 `action.log`（外部 process 寫入前綴 `▌ ` 紫色顯示）。單 session 視角；多 session dashboard 已封存（要復活見 `internal/_tui_archived/sessionObserver.go`）。

## TUI slash command

| 指令 | 說明 |
|---|---|
| `/switch` | 切換 session（picker，當前預選；最末 `(new session)` sentinel）。 |
| `/new [name]` | 建立 session；帶 name 即固定登錄（衝突檢查）。 |
| `/bot [name body...]` | 編輯 bot persona — 兩段 popup（name → multiline body），或 `parts≥3` 走 inline 直存。 |
| `/model [global\|session]` | `global` → 加／刪 registry；`session` → 從 `cfg.Models` 挑一個。 |
| `/mcp [add\|remove]` | MCP server 設定串接 popup；改動須重啟 daemon 才生效。 |
| `/dispatcher-model` | 從 `cfg.Models` 挑 dispatcher model。 |
| `/summary-model` | 從 `cfg.Models` 挑 summary 用的 model（選 `(use dispatcher)` 即回退到 dispatcher）。 |
| `/reasoning [global\|session]` | `low` / `medium` / `high`。 |
| `/discord [enable\|disable]` | 啟用／停用 Discord bot（in-TUI popup chain：token 輸入 → 驗證 → keychain 寫入 → daemon fsnotify reload）。 |
| `/telegram [enable\|disable]` | 啟用／停用 Telegram bot（與 `/discord` 同模式的 in-TUI popup chain；首次與 bot 對話的 chat 必須通過 6 碼 in-chat OTP，授權清單存於 `~/.config/agenvoy/.telegram`）。 |
| `/kuradb [enable\|disable]` | 切換 KuraDB RAG。`enable` 透過 popupText 收 `OPENAI_API_KEY` → `tea.ExecProcess` 跑 `curl https://cloud.agenvoy.com/KuraDB/install.sh \| bash`（TTY 還給 child 跑 `sudo` + apt/brew）→ 寫 `kuradb_enabled=true`。`disable` 跑 `sudo rm /usr/local/bin/kura` + 清 flag。Daemon 透過 fsnotify 接到後 spawn／kill child。詳見 [KuraDB RAG](KuraDB-RAG.zh.md)。 |
| `/cron [add\|remove\|edit]` | 週期排程。`add` → 多行 textarea 取需求 → 派 `/scheduler-skill-creator <需求>`（skill 缺 when/what 透過 `ask_user` 補問）。`remove` → 列出 → 確認 popup → `runtime.RemoveCron` + 移 skill 目錄至 `.Trash`。`edit` → 列出 → 取需求 → agent 自選走 `patch_cron` 或重寫 SKILL body。Picker **session-scoped** —— 只顯示 `session_id == currentSessionID` 的 entry。 |
| `/task [add\|remove\|edit]` | 一次性排程（鏡像 `/cron`；使用 `add_task` / `patch_task` / `remove_task`）。Session-scoped picker。 |
| `/sched-<name>` | 顯示於 slash picker 最末（warn-purple label）；選取後派該 scheduler skill 的 body，dispatch 加入「執行 已存在 schedule，不要 activate creator」preamble。依 session 過濾 —— 只列出綁定當前 session task／cron 的 skill。 |
| `/allow-skill` | 兩層 popup（scope → skill 列表 with `✓` 標記），將 skill 標為 always-allow。寫入 `<workDir>/.agenvoy/allow_skill`（project）或 `~/.config/agenvoy/allow_skill`（global）。Always-allow 在 exec 起點將 `AllowAll=true` —— 跳過 confirm gate，但 sandbox／validator 照常。**危險**：只標你信任的 skill。 |
| `/mode [cli\|web]` | TUI 渲染 vs 瀏覽器頁面（瀏覽器是 `update_page` 的 session canvas，由 REST 提供）。 |
| `/history` | 重載可見 transcript —— 清螢幕、reprint header、從 session `action.log` 渲染最近 100 筆。 |
| `/log` | 在 `$PAGER`（fallback `less -Rf +G`，跳底）開啟 raw `action.log`。`\x1F` marker 展開回 newline 以利閱讀。 |
| `/cmd` | 在當前 workDir 直接跑 shell 指令（`sh -c`）。 |
| `/update` | 確認 popup → `tea.ExecProcess` 跑 `agen stop && agen update` → 退出（重 `agen` attach 拿新 binary）。 |
| `/clear` | 清視窗顯示；記憶不動。 |
| `/exit`, `/quit` | 退出 TUI。 |

## 輸入前綴

`exec.Run()` 解析順序（僅 CLI / TUI / Telegram；Discord 與 HTTP 不解析 `:name`）：

1. **`:name`** —— session override（一次性派遣，不改主指標）
2. **`MatchExternal`** —— 外部 CLI agent 派發（`/claude`、`/codex` 等）
3. **`MatchSkillCall`** —— skill 啟用（`/<skill-name>`）

### `:name` Session Override

```bash
make cli ":ship-v0.20 /commit-generate"
```

可與 skill、external agent 組合 —— 由左至右解析（`:bot` → external → skill → execute）。

### 外部 CLI 前綴

| 前綴 | 模式 | 底層 flag |
|---|---|---|
| `/claude` | Read-only | `claude -p --disallowedTools=Edit,Write,NotebookEdit` |
| `/claude-allow` | Write | `claude -p --permission-mode acceptEdits` |
| `/codex` | Read-only | codex CLI（預設 sandbox）+ `--output-last-message` + `--skip-git-repo-check` |
| `/codex-allow` | Write | codex CLI `--dangerously-bypass-approvals-and-sandbox` |
| `/gh` 或 `/copilot` | Read-only | `gh copilot -s`（無寫入變體） |
| `/gemini` | Read-only | `gemini --approval-mode plan --skip-trust` |
| `/gemini-allow` | Write | `gemini --yolo --skip-trust` |

### Skill 前綴

`extensions/skills/<name>/` 下的 skill 用 `/<name>` 觸發：

```bash
make cli "/commit-generate"
make cli "/readme-generate private MIT"
```

`/<skill-name>` 後的 args 作為 binding context 傳遞 —— 見 [Skill 系統 → User message 是 binding context](Skill-System.zh.md#user-message-是-binding-context)。

## REST API

`make app` 啟動（預設 port `:3000`）。

| Endpoint | 說明 |
|---|---|
| `POST /v1/send` | 送訊息；body `{sid?, persist?, text}` |
| `POST /v1/key` | 寫入 keychain |
| `GET /v1/key` | 讀取 keychain |
| `GET /v1/tools` | 列出已註冊 tool |
| `POST /v1/tool/:tool_name` | 直接呼叫 tool |
| `GET /v1/session/:sid/status` | 讀 `status.json`（session 不存在回 404） |
| `GET /v1/session/:sid/log` | SSE 串流 `action.log`（1 s ticker、連 15 tick 無事件送 `: ping`） |

`POST /v1/send` 語意：

| `persist` | `sid` | 結果 |
|---|---|---|
| `false`（預設） | 空 | 建 `temp-<uuid>`，1 h idle 清除 |
| `true` | 空 | 建 `http-<uuid>`，永久保留 |
| 任意 | 已給 | 用該 sid（`persist` 忽略） |

## 環境變數

完整清單見 [設定檔](Configuration.zh.md)。
