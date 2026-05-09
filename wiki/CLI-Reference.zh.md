# 命令列參考

> [English](https://github.com/agenvoy/Agenvoy/wiki/CLI-Reference)

## 頂層派發

`agen` 解析 `os.Args[1]` 並派發到六個 sub-group 之一；無 subcommand 啟動完整 app（`runApp`）。

```bash
agen                                           # TUI + Discord + REST 完整堆疊
agen model   {add|remove|list|planner|reasoning}
agen skill   {list}
agen session {new|switch|config} [name]
agen mcp     {list|add|remove}
agen cli     <input...>                        # 單次，每 tool 需 confirm
agen run     <input...>                        # 單次，所有 tool 自動放行
```

### `agen model`

| Subcommand | 行為 |
|---|---|
| `add` | 互動式新增 provider／model（憑證寫入 keychain） |
| `remove`（alias `rm`） | 互動式移除 provider／model |
| `list` | 列出已註冊 model |
| `planner` | 選 planner model |
| `reasoning` | 設 planner 推理層級：`low` / `medium` / `high` / `xhigh` |

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

### `agen skill`

`agen skill`（無 subcommand）與 `agen skill list` 都列出 `extensions/skills/` 下可用 skill。

## `make` 捷徑

```bash
make build                      # 編譯並安裝至 /usr/local/bin/agen
make app                        # 完整堆疊（TUI + Discord + REST API）
make discord                    # 舊版 Discord-only server
make cli <input...>             # = agen cli <input...>
make run <input...>             # = agen run <input...>
make model   [add|remove|list|planner|reasoning]
make skill   [list]
make session [new|switch|config] [name]
make mcp     [list|add|remove]
make test                       # go test ./test/... -v -timeout 60s
```

## TUI 快捷鍵

主畫面（`Content` / `Logs`）：

| 鍵 | 行為 |
|---|---|
| `i` | 開啟 Message 輸入（`> ` prompt、多行；支援 modifier 的終端 `Shift+Enter` 插入換行） |
| `c` | 開啟 Command 輸入（`$ ` prompt、單行） |
| `Enter` | 送出（Message：送出 + 清空；Command：執行） |
| `Esc` | 關閉輸入 / popup |
| `Tab` | 切換 Content / Logs；在 input pages 內切換 Command ↔ Message |
| `Ctrl+P` | 切換 co-work dashboard（Sessions / Log / Pending 三 panel） |
| `h` / `l` / 方向鍵 | 切換 panel |
| `j` / `k` | 滾動當前 view |
| `Ctrl+C` | 取消當前執行 |

Co-work dashboard：

- **Sessions** —— 左 pane，列出追蹤中的 `cli-*` 與 `http-*`（不含 `temp-*` / `dc-*`）
- **Log** —— 中間，依 CLI 風格 tail 選取 session 的 `action.log`
- **Pending** —— 右 pane，僅當 `pending.Snapshot()` ≥1 entry 才顯示；選取 entry 自動跳 Sessions 至該 sid

## 輸入前綴

`exec.Run()` 解析順序（僅 CLI / TUI；Discord 與 HTTP 不解析 `:name`）：

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

`/<skill-name>` 後的 args 作為 binding context 傳遞 —— 見 [Skill 系統 → User message 是 binding context](https://github.com/agenvoy/Agenvoy/wiki/Skill-系統#user-message-是-binding-context)。

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

完整清單見 [設定檔](https://github.com/agenvoy/Agenvoy/wiki/設定檔)。
