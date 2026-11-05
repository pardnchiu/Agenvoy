---
name: scheduler-skill-creator
description: |
  建立並排程定時觸發的 skill。**所有新增定時／週期任務、提醒、排程通知的請求必須走此 skill**，禁止直接呼叫 schedules(mode=write)（那是 skill 已存在時的時間綁定工具，不該作為新建排程的入口）。

  必定觸發的訊息特徵（任一即活化）：
  - 相對延遲：「X 分鐘後」「X 小時後」「稍後」「待會」「等一下」
  - 明確時間：「X 點」「下午 X 點」「明天 X 點」「後天」「YYYY-MM-DD HH:MM」
  - 週期性：「每 X 分鐘」「每小時」「每天」「每週」「每月」「定時」「固定」
  - 提醒 / 通知意圖：「提醒我」「通知我」「告訴我」+ 時間描述

  範例觸發訊息：「5 分鐘後提醒我喝水」「每天早上 9 點抓 HN 頭條」「明天下午 3 點開會」「每 5 分鐘查台積電股價」。

  **不觸發**（即使訊息含「觸發」「排程」字眼也不 activate）：
  - 訊息含 `[執行已存在 scheduler skill:` 標記 → 為 `/sched-<name>` 手動 trigger，當前 agent 直接執行 body
  - 訊息為一份完整的 SKILL.md body（`# Title` + `## 任務` + `## 輸出格式` 結構），無建立／排程動詞 → 為 skill execution，非 creation
  - 訊息僅含「執行 skill X」「跑 X」「run skill X」無時間 token → 為 execution

  流程：解析訊息抽出「要做什麼」「何時觸發」→ 缺項用 ask_user 補問 → 生成 skill 檔案至 ~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md（無 scheduler- 前綴，hash 用於避免命名衝突）→ 呼叫 schedules(mode=write) 綁定時間 → 回報。
---

> **本 Skill 為 Agenvoy 內部最佳化版本**，依 Agenvoy 的執行環境撰寫（`run_command` 的 CWD、`~/.config/agenvoy/skills/.system/` 安裝位置、`edit_skill`／`schedules`／`find_edit_tool` 等工具、subagent 與排程的觸發路徑），**不保證適配其他 AI harness**。

# Scheduler Skill 建立器

## 目的

scheduler 採 skill-based 觸發：到時間時，daemon 讀 `scheduler/<short>/SKILL.md` body 並起 in-process subagent 跑（always-allow）。本 skill 的職責 = 「從使用者意圖建出 skill 並綁定時間」，完整跑完 6 步即完成排程。

**重要：scheduler 用 skill 與一般 skill 隔離**

| 比較 | 一般 skill | scheduler 用 skill |
|---|---|---|
| 路徑 | `~/.config/agenvoy/skills/<name>/SKILL.md` | `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md` |
| frontmatter `name` | `<name>` | `<short>-<hash8>` (**無前綴**) |
| 一般 `/<name>` 補全 | 出現 | **不出現**（scanner 不掃 scheduler/） |
| 呼叫方式 | `/<name>` 觸發 | `schedules(mode=write, target=task, skill_name=<short>-<hash8>)` / `schedules(mode=write, target=cron, skill_name=<short>-<hash8>)` |

## 成功標準

- 生成檔案: `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`，frontmatter `name: <short>-<hash8>`（無前綴）
- skill body 描述任務行為、引用具體 tool
- 呼叫 `schedules(mode=write, target=task, time, skill_name=<short>-<hash8>)` 或 `schedules(mode=write, target=cron, time, skill_name=<short>-<hash8>)` 綁定時間成功
- 回報生成位置、full name（含 hash）、排程類型（one-shot／recurring）、下次觸發時間

## 步驟

### 0. 時間檢查門檻（**強制首動作**）

在呼叫**任何其他 tool**（特別是 `run_command` 跑 init script）**之前**，先檢查使用者訊息**是否含明確時間 token**。時間 token 定義：

| 類別 | Token 範例 |
|---|---|
| 相對延遲 | `N 分鐘後`／`N 小時後`／`N 秒後`／`待會`／`稍後`／`等一下` |
| 絕對時鐘 | `X 點`／`HH:MM`／`下午 X 點`／`晚上 X 點` |
| 絕對日期 | `今天`／`明天`／`後天`／`YYYY-MM-DD` |
| 週期 | `每 N 分`／`每小時`／`每天`／`每週`／`每月`／`定時`／`固定` |

**判定流程**（兩個 yes／no 各自獨立檢查；缺項一律走 `ask_user` **tool call**）：

1. **任務 token 存在？**（訊息含可執行動作描述）
   - 否 → 呼叫 `ask_user` tool：`{"questions":[{"question":"要做什麼？例：抓 HN 頭條 / 提醒我喝水"}]}`
2. **時間 token 存在？**（上表任一）
   - 否 → 呼叫 `ask_user` tool：`{"questions":[{"question":"什麼時候執行？例：5 分鐘後 / 每 5 分鐘 / 明天 9 點"}]}`

兩項都缺時，同一個 `ask_user` 的 `questions` 帶兩題送出。收到回傳的 `answers` 後，把答案併入原訊息重跑步驟 0；兩者都齊才進步驟 1。

**問題一律用 `ask_user` tool call 送出**：`ask_user` 走 `pending.Ask` 阻塞等待 reply，harness 開 popup／prompt 收答案，agent 收到結構化 `answers` 後接著執行。TUI／CLI／Web／Telegram／Discord 都支援；只有 chat completions 端點沒有這個通道。

**需要補問的例子**：

| 訊息 | 為何要 ask_user |
|---|---|
| 「說我很棒」「提醒我」「叫我喝水」 | 任務有，時間**無** |
| 「等等」「之後」「找時間」 | 模糊詞不算明確 token |
| 「下班後」「有空時」 | 無可正規化為 cron／datetime 的時間值 |

時間以使用者說的為準：沒說就用 `ask_user` 問，不用預設值（`+10m`、`09:00`）或推測補齊。

### 1. 解析需求

步驟 0 通過後（任務與時間都齊全），抽兩元素：

- **任務**：要做什麼（行為描述）
- **時間**：何時觸發

範例解析：

| 訊息 | 任務 | 時間 |
|---|---|---|
| 每 5 分鐘提醒我台積電最新股價 | 查台積電股價並提醒 | 每 5 分鐘（recurring） |
| 明天早上 9 點提醒我開會 | 開會提醒 | 明天 09:00（one-shot） |
| 5 分鐘後叫我喝水 | 喝水提醒 | +5m（one-shot） |
| 每天抓 HN 頭條給我 | 抓 HN 頭條摘要 | 每天（recurring，**步驟 0 已要求補問時段**） |

缺項一律以 `ask_user` **tool call** 補齊；`questions` 是 array，當下所有缺項寫成多題一起送。

### 2. 時間正規化 + 選 tool

| 使用者說 | 工具 | `time` 參數 |
|---|---|---|
| `X 分鐘後` | `schedules(mode=write, target=task)` | `+Xm` |
| `X 小時後` | `schedules(mode=write, target=task)` | `+Xh` |
| `今天 X 點`（24h） | `schedules(mode=write, target=task)` | `HH:MM` |
| `明天 / 特定日期 X 點` | `schedules(mode=write, target=task)` | `YYYY-MM-DD HH:MM` |
| `每 X 分鐘` | `schedules(mode=write, target=cron)` | `*/X * * * *` |
| `每小時` | `schedules(mode=write, target=cron)` | `0 * * * *` |
| `每天 X 點` | `schedules(mode=write, target=cron)` | `MM HH * * *` |
| `每週 N`（0=Sun, 1=Mon, ..., 6=Sat） | `schedules(mode=write, target=cron)` | `MM HH * * N` |
| `每月 D 日 X 點` | `schedules(mode=write, target=cron)` | `MM HH D * *` |

決定走 `schedules(mode=write)` target=task（一次性）或 target=cron（週期）。

### 3. 初始化 skill 目錄（**強制走 init 腳本**）

> **腳本路徑**：`run_command` 的 CWD 是使用者的工作目錄，**不是本 skill 目錄**，相對路徑 `scripts/...` 必定找不到（實測會讓 agent 反覆 glob 找檔案，白燒數輪）。本 skill 只服務 Agenvoy、安裝位置固定，一律用絕對路徑 `~/.config/agenvoy/skills/.system/scheduler-skill-creator/scripts/`。

> **禁止直接用 `edit_skill(mode=write)`／`edit_file(mode=write)` 建立 SKILL.md** —— LLM 容易寫成 `<short>.md` 而非 `<short>/SKILL.md`，或誤加 `scheduler-` 前綴；也無法自行產生 hash suffix。必須先跑 init 腳本。

用 `run_command` 執行：

```bash
python3 ~/.config/agenvoy/skills/.system/scheduler-skill-creator/scripts/init_scheduler_skill.py <short-name>
```

`<short-name>` 由步驟 1 的任務描述推導（kebab-case、**不含 `scheduler-` 前綴**、**不含 hash**）。腳本會：

- 正規化 short name（lowercase、hyphen-case）
- 產生 8-char hex random suffix（`secrets.token_hex(4)`），組成 full name `<short>-<hash8>`
- 建立 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`，寫入含 frontmatter `name: <short>-<hash8>` 的 TODO 模板

**捕捉 full name**：stdout 會印一行 `[OK] skill name: <short>-<hash8>`，**完整字串**（含 hash）是後續步驟 4／5 要用的 `skill_name`。極罕見 hash 碰撞時印 `[ERROR] collision` exit 1，重跑一次即可。

**重綁定既有 skill 的時間**（user 說「把那個 X 改成 Y」）：不再跑 init 腳本，直接用既存 full name 進步驟 5；既存 full name 可從先前回報訊息找，或 `find_files(mode=list)` 列出 `~/.config/agenvoy/skills/scheduler/` 選擇。

### 4. 建構 skill 內容（**委派 `/skill-creator`**）

目錄與名稱在步驟 3 已經定案，這一步只做內容。**呼叫 `/skill-creator`**，用它的「編輯現有 Skill」路徑填內容，不要在這裡自己重寫一套設計流程。

```
/skill-creator 編輯現有 skill：~/.config/agenvoy/skills/scheduler/<short>-<hash8>/
任務：<步驟 1 收集到的行為細節>
```

**交給 `/skill-creator` 的部分**（照它的步驟走）：

| 它的步驟 | 在這裡的作用 |
|---|---|
| 一：透過具體範例理解 | 已由步驟 1 完成，把結果直接給它，**不要再問一次** |
| 二：規劃可重用內容 | 決定要不要 `scripts/` |
| **二點五：工具／Skill 搭配探索** | 讀 `## Skills` → `find_edit_tool(mode=search)` → 都沒有才寫 `scripts/*.py` |
| 四：編輯 | `edit_skill(mode=patch)` 取代模板的 `[TODO: ...]` |

**這裡的額外約束**（`/skill-creator` 不知道排程的規則，必須由你把關）：

- **不准跑 `init_skill.py`**（它會用自己的命名規則在 `skills/` 底下另開一個目錄）。目錄已存在，走「編輯現有 Skill」路徑
- **不准改名、不准搬位置** —— 名稱固定 `<short>-<hash8>`，位置固定 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`
- **不准跑步驟五（打包）** —— 排程 skill 不外流
- **body 引用的 skill／tool 必須確認存在**：skill 以 system prompt 的 `## Skills` 清單為準（那份清單已在 context 裡，**不要用 `run_skill` activate 驗證** —— 每次 activate 都是一輪往返加一整份 SKILL.md 進 context）；tool 名稱以 `find_edit_tool(mode=search)` 的回傳為準。觸發時 subagent 找不到會直接 abort，使用者拿不到結果也看不到原因
- **`scripts/` 寫進 `scheduler/<short>-<hash8>/scripts/`**，不用 `edit_tool` 產全域工具

**必填欄位**：

- `description:` ← 步驟 1 的「一句話描述」
- `## 任務` ← 步驟 1 的「行為細節」，引用已確認存在的 skill／tool
- `## 輸出格式` ← 期望輸出形式

**禁止**在 skill body 內加任何「推送到 channel」「呼叫 http_request 給 Discord」「呼叫 MCP discord tool」之類的 notify 指令 —— scheduler 觸發後 runtime 自動把輸出送回原 caller channel（Discord 來源送回原頻道、CLI／HTTP 來源送回 action.log）。Skill body 只需專注產出**任務結果文字**。

### 5. 綁定時間

依步驟 2 結果呼叫，`skill_name` 用步驟 3 stdout 印出的完整 `<short>-<hash8>`：

```
schedules(mode=write)(target="task", time="<time_value>", skill_name="<short>-<hash8>")
# 或
schedules(mode=write)(target="cron", time="<cron_expression>", skill_name="<short>-<hash8>")
```

`skill_name` **不加 `scheduler-` 前綴**（內部會直查 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md` 確認存在）。session_id 內部自動取 caller `e.SessionID`，不必傳。

成功會回 `ID: <hash>` 等資訊。失敗（skill 不存在、cron 表達式錯誤、`time` 已過）就 abort 本流程，向使用者回報原因。

### 6. 回報

簡短告知：

- skill 已建立: `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`
- skill name: `<short>-<hash8>`（無前綴，hash 自動產生避免命名衝突）
- 排程: `schedules(mode=write)` 的回應內容（含下次觸發時間、ID）

## 命名規則

| 項目 | 規則 | 範例 |
|---|---|---|
| short name（輸入 init script） | lowercase / hyphen-case，無 `scheduler-` 前綴、無 hash | `daily-hn-digest`、`tsmc-stock-watch` |
| hash suffix | init script 產生的 8-char hex random | `a3f9b2c1` |
| full name（檔案／frontmatter／schedules(mode=write) skill_name 用） | `<short>-<hash8>` | `tsmc-stock-watch-a3f9b2c1` |
| 目錄 | `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/` | `.../tsmc-stock-watch-a3f9b2c1/` |

**禁止**在任何環節加 `scheduler-` 前綴。`scheduler` 已表達於目錄路徑，加前綴只會造成 `scheduler/scheduler-foo-<hash>/` 之類的重複命名。

**禁止**自行產生／猜測 hash suffix。Hash **必須**由 init script 用 `secrets.token_hex(4)` 隨機生成，LLM 從 stdout 抓 `[OK] skill name:` 那行的值即可。

## 輸出路由（runtime 自動處理）

scheduler 觸發後，runtime 會把 subagent 產出的最終文字自動送回 caller 端：

| Caller session prefix | 路由行為 |
|---|---|
| `dc-*`（Discord） | 自動 `ChannelMessageSend` 回原頻道（含 ` - <skill 短名>` 標籤） |
| `cli-*`／`http-*`／TUI 觸發 | 留在 session history／action.log，由 caller 端工具讀取 |

**所以 skill body 不需要、也禁止**寫「推送到 channel」「呼叫 `http_request` 發 Discord webhook」「呼叫 MCP discord tool」之類的 notify 指令。寫了會在觸發時造成多餘的 token 與認證錯誤（subagent 沒 `DISCORD_BOT_TOKEN` 互動環境）。

## Secret／API Key（skill body 引用 token 時必看）

被觸發的 scheduler skill 跑在獨立 subagent，**不持有任何明文 secret**。若 body 內呼叫的 tool（如 `http_request`、自製 api_tool、script_tool）需要 API token：

- **命名格式**：`{品牌}_API_KEY`（SCREAMING_SNAKE_CASE），例 `OPENAI_API_KEY`、`CODEX_API_KEY`、`POLYGON_API_KEY`、`STAGING_API_KEY`
- **儲存位置**：macOS keychain 中 **service = `agenvoy`**、**account = key 名**，組合識別 `agenvoy.{key}`（例 `agenvoy.OPENAI_API_KEY`）
- **取值方式**：
  - api_tool：`auth.env: "<KEY_NAME>"`（schema 只記 key 名，無 `agenvoy.` 前綴）
  - script_tool：讀 OS keychain（service `agenvoy`）—— macOS `security find-generic-password -s agenvoy -a <KEY_NAME> -w`；Linux `secret-tool lookup service agenvoy account <KEY_NAME>`（key 名同樣不帶前綴）
  - skill body 純文字：直接引用 tool，**不**在 SKILL.md 寫明文 token、**不**寫 `export ENV=value` 之類指令
- **缺 key 處置**：若觸發時 keychain 無對應 key，subagent 會在 tool 端拿到 401／空值錯誤；skill body 不負責「補登」，請使用者預先用 `store_secret` 落地

**禁止**在 scheduler skill 的 SKILL.md frontmatter／body 任何位置寫死 token 值或要求使用者在 cron 觸發時互動輸入 — subagent 無對話環境，不可能收 plaintext。

## 時間敏感性提醒（寫入 skill body 時注意）

被觸發的 skill 跑在獨立 subagent session，**沒有當下對話上下文**。skill body 必須：

- 不依賴「使用者剛才說了什麼」
- 不假設特定變數已被定義
- 引用具體 tool 名稱與參數（自包含可重現）
- cron 觸發時反覆執行，邏輯應 idempotent 或自帶 dedup

## 完整範例

使用者：「每 5 分鐘提醒我台積電最新股價」

**步驟 1** 解析：任務 = 查 2330.TW 股價；時間 = 每 5 分鐘 → recurring。兩者皆有，不問。

**步驟 2** 正規化：`schedules(mode=write)(target="cron", time="*/5 * * * *", ...)`

**步驟 3** `run_command python3 ~/.config/agenvoy/skills/.system/scheduler-skill-creator/scripts/init_scheduler_skill.py tsmc-stock-watch`

stdout：
```
[OK] created   : /Users/.../skills/scheduler/tsmc-stock-watch-a3f9b2c1/SKILL.md
[OK] skill name: tsmc-stock-watch-a3f9b2c1
...
```

抓出 full name `tsmc-stock-watch-a3f9b2c1`。

**步驟 4** `edit_skill(mode=patch)` 填入（frontmatter `name` 用 full name）：

```markdown
---
name: tsmc-stock-watch-a3f9b2c1
description: 每 5 分鐘抓取台積電 2330.TW 即時股價並提醒。
---

# Tsmc Stock Watch

## 任務

透過 `search_web` 找到 `2330.TW` 最新報價來源，再用 `fetch_page` 讀取結果。

## 輸出格式

`台積電 2330.TW: NT$<price> (<change>% 從昨收)` 一行。
```

**步驟 5** `schedules(mode=write)(target="cron", time="*/5 * * * *", skill_name="tsmc-stock-watch-a3f9b2c1")`

**步驟 6** 回報：「已排程每 5 分鐘觸發 `tsmc-stock-watch-a3f9b2c1`。下次觸發 HH:MM。」

## 不做的事

- **不**用 `edit_skill(mode=write)`／`edit_file(mode=write)` 直接建立 SKILL.md —— 必須走 `init_scheduler_skill.py`，避免結構錯誤（`<name>.md` vs `<name>/SKILL.md`）
- **不**在 short name、frontmatter、skill_name 任何位置加 `scheduler-` 前綴
- **不**留 `[TODO: ...]` 佔位符在最終 skill —— 步驟 4 須把所有 TODO 替換為具體內容
- 時間以使用者說的為準；沒說就用 `ask_user` 問，不用預設值或推測補齊
- **不**跳過步驟 5 的 `schedules(mode=write)` —— skill 建立但沒綁時間 = 排程不會觸發
- **不**在 body 引用未經 `find_edit_tool(mode=search)` 確認存在的 tool name —— 觸發時 subagent 找不到 tool 會直接 abort，使用者拿不到結果也看不到錯誤原因
- **不**用 `edit_tool` 產全域 `script_*` 工具 —— 排程要的腳本寫進自己的 `scripts/`（步驟 4），全域工具是 tool generate 的職責，兩者不混用
- **不**讓步驟 4 委派出去的 `/skill-creator` 跑 `init_skill.py` 或 `package_skill.py` —— 前者會用它自己的命名規則在 `skills/` 底下另開目錄（排程綁的是 `scheduler/<short>-<hash8>`，綁不到就不會觸發），後者產出的 `.skill` 排程用不到
