---
name: scheduler-skill-creator
description: |
  建立並排程定時觸發的 skill。**所有新增定時／週期任務、提醒、排程通知的請求必須走此 skill**，禁止直接呼叫 add_task / add_cron（那是 skill 已存在時的時間綁定工具，不該作為新建排程的入口）。

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

  流程：解析訊息抽出「要做什麼」「何時觸發」→ 缺項用 ask_user 補問 → 生成 skill 檔案至 ~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md（無 scheduler- 前綴，hash 用於避免命名衝突）→ 呼叫 add_task 或 add_cron 綁定時間 → 回報。
---

# Scheduler Skill 建立器

## 目的

scheduler 採 skill-based 觸發：到時間時，daemon 讀 `scheduler/<short>/SKILL.md` body 並起 in-process subagent 跑（always-allow）。本 skill 的職責 = 「從使用者意圖建出 skill 並綁定時間」，完整跑完 6 步即完成排程。

**重要：scheduler 用 skill 與一般 skill 隔離**

| 比較 | 一般 skill | scheduler 用 skill |
|---|---|---|
| 路徑 | `~/.config/agenvoy/skills/<name>/SKILL.md` | `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md` |
| frontmatter `name` | `<name>` | `<short>-<hash8>` (**無前綴**) |
| 一般 `/<name>` 補全 | 出現 | **不出現**（scanner 不掃 scheduler/） |
| 呼叫方式 | `/<name>` 觸發 | `add_task(skill_name=<short>-<hash8>)` / `add_cron(skill_name=<short>-<hash8>)` |

## 成功標準

- 生成檔案: `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`，frontmatter `name: <short>-<hash8>`（無前綴）
- skill body 描述任務行為、引用具體 tool
- 呼叫 `add_task(time, skill_name=<short>-<hash8>)` 或 `add_cron(time, skill_name=<short>-<hash8>)` 綁定時間成功
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

收到 `ask_user` 回傳的 `answers` 後，把答案併入原訊息重跑步驟 0；兩者都齊才進步驟 1。

**強制使用 tool call、禁止用純文字輸出問題**：

- ❌ 輸出 `什麼時候執行？例：5 分鐘後 / ...` 作為 assistant 文字回應 → **流程中斷**（TUI／CLI 不會把使用者下一句話視為這題的回答）
- ✅ 呼叫 `ask_user` tool 帶 `questions` → harness 開 popup／prompt 收答案，回到 agent 主迴圈繼續

**為何**：`ask_user` tool 走 `pending.Ask` 阻塞等待 reply，agent 自動收到結構化 `answers` 後續執行；純文字輸出則 turn 結束、context 不接續，使用者下次輸入會被視為**新任務**而非答案。

**反例**（這些**必須**先 `ask_user` tool call 補時間，禁止直接進 init）：

| 訊息 | 為何要 ask_user |
|---|---|
| 「說我很棒」「提醒我」「叫我喝水」 | 任務有，時間**無** |
| 「等等」「之後」「找時間」 | 模糊詞不算明確 token |
| 「下班後」「有空時」 | 無可正規化為 cron／datetime 的時間值 |

**禁止行為**（違反視為流程失敗）：

- ❌ 訊息無時間 token 仍跑 `run_command python3 .../init_scheduler_skill.py`
- ❌ 用「+10m」「+5m」「+1h」當預設值補齊未指定的時間
- ❌ 推論「使用者大概是想要 N 分鐘後」之類腦補
- ❌ 缺時段（如「每天」沒說幾點）時自動填「09:00」
- ❌ **以純文字輸出問題替代 `ask_user` tool call**（mini model 易犯，違反「ambiguity 用 tool 而非 text」）

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

一次 `ask_user` 一題，依需要追問。**禁止假設**。

### 2. 時間正規化 + 選 tool

| 使用者說 | 工具 | `time` 參數 |
|---|---|---|
| `X 分鐘後` | `add_task` | `+Xm` |
| `X 小時後` | `add_task` | `+Xh` |
| `今天 X 點`（24h） | `add_task` | `HH:MM` |
| `明天 / 特定日期 X 點` | `add_task` | `YYYY-MM-DD HH:MM` |
| `每 X 分鐘` | `add_cron` | `*/X * * * *` |
| `每小時` | `add_cron` | `0 * * * *` |
| `每天 X 點` | `add_cron` | `MM HH * * *` |
| `每週 N`（0=Sun, 1=Mon, ..., 6=Sat） | `add_cron` | `MM HH * * N` |
| `每月 D 日 X 點` | `add_cron` | `MM HH D * *` |

決定走 `add_task`（一次性）或 `add_cron`（週期）。

### 3. 初始化 skill 目錄（**強制走 init 腳本**）

> **禁止直接用 `write_file` 建立 SKILL.md** —— LLM 容易寫成 `<short>.md` 而非 `<short>/SKILL.md`，或誤加 `scheduler-` 前綴；也無法自行產生 hash suffix。必須先跑 init 腳本。

用 `run_command` 執行：

```bash
python3 scripts/init_scheduler_skill.py <short-name>
```

`<short-name>` 由步驟 1 的任務描述推導（kebab-case、**不含 `scheduler-` 前綴**、**不含 hash**）。腳本會：

- 正規化 short name（lowercase、hyphen-case）
- 產生 8-char hex random suffix（`secrets.token_hex(4)`），組成 full name `<short>-<hash8>`
- 建立 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`，寫入含 frontmatter `name: <short>-<hash8>` 的 TODO 模板

**捕捉 full name**：stdout 會印一行 `[OK] skill name: <short>-<hash8>`，**完整字串**（含 hash）是後續步驟 4／5 要用的 `skill_name`。極罕見 hash 碰撞時印 `[ERROR] collision` exit 1，重跑一次即可。

**重綁定既有 skill 的時間**（user 說「把那個 X 改成 Y」）：不再跑 init 腳本，直接用既存 full name 進步驟 5；既存 full name 可從先前回報訊息找，或 `list_files ~/.config/agenvoy/skills/scheduler/` 列出選擇。

### 3.5 工具／skill 搭配探索（步驟 4 前置）

填 skill body 之前**必須**確認會用到的 skill／tool 真實存在，否則觸發時 subagent 找不到 → 直接 abort、使用者拿不到結果。

**Skill 優先於 tool**：skill 是預先封裝好的高階流程（含 prompt 規則／步驟／格式），tool 是低階呼叫；同樣的任務若有對應 skill，body 寫 `/<skill-name>` 比直接組 tool call 更穩定且符合既有設計。

**強制探索順序**（**禁止跳順序、禁止只跑其中一步**）：

1. **讀 system prompt 的 `## Skills` 區段**（你的 context 內已有）：把使用者意圖（步驟 1 的「任務」）對照所有 skill 的 `description`，列出**任何描述提及相關主題的候選**。例：
   - 「分析比特幣」「BTC 價格」→ `bitcoin-lookup`（描述含「BTC／Bitcoin／比特幣價格／行情／分析」）
   - 「彙整 commit 訊息」→ `commit-generate`
   - 「跑程式碼 review」→ `code-reviewer`
2. **逐個 `activate_skill` 驗證**候選：activate 成功代表存在，body 改寫成 `任務：呼叫 /<skill-name> 觸發本任務`。失敗（skill 不存在）才往下一步。
3. **無匹配 skill 時，`search_tools` 找 raw tool**：抽出步驟 1 任務的動詞，對每個動詞呼一次 `search_tools`。回傳的 tool name 才能寫進 body：

   ```
   search_tools({"query": "fetch stock price", "max_results": 5})
   search_tools({"query": "yahoo finance", "max_results": 5})
   ```

4. **`search_tools` 也找不到**（例：使用者要求「打卡」但無此 tool 也無對應 skill）→ 回 `ask_user`：「目前環境沒有可完成 X 的 skill／tool，可以改成 Y 嗎？」。**禁止**寫不存在的 skill／tool name 進 body。

**判定原則**：

| 情境 | body 怎麼寫 |
|---|---|
| 有 skill 命中（步驟 2 activate 成功）| `任務：呼叫 /<skill-name>，把結果整理成「## 輸出格式」要求的形式` |
| 無 skill 但有 tool（步驟 3 search 命中）| `任務：呼叫 <tool-name>，參數 ...` |
| 兩者都無（步驟 4）| 中止 init，先 `ask_user` 確認替代方案 |

**常用 skill／tool 速查**（先想想再去 activate／search）：

| 任務類型 | 候選 skill（優先） | 候選 tool（退一步） |
|---|---|---|
| 比特幣行情／分析 | `bitcoin-lookup` | `fetch_yahoo_finance`（BTC-USD）|
| 一般股價／財經 | （視 `## Skills` 是否有對應）| `fetch_yahoo_finance` |
| HN／RSS 摘要 | （視是否有 digest skill）| `fetch_google_rss` |
| 影片字幕 | — | `fetch_youtube_transcript` |
| 網頁／API 抓取 | — | `fetch_page`／`send_http_request`／`api_*` |
| 程式碼 review | `code-reviewer` | — |
| Commit／版號 | `commit-generate`／`version-generate` | — |
| 計算 | — | `calculator` |
| 純文字提醒（無 IO） | — | 不需 tool，body 直接寫死要輸出的文字 |

**`scheduler-skill-creator` 與 `scheduler/` 下任何 skill 不算候選** —— 前者是本流程自己、不能遞迴；後者是 scheduler 用內部 skill（透過 `add_task` 綁定觸發、不能用 `/<name>` 從 body 呼叫）。

### 4. 填充 skill body

用 `patch_file` 取代模板中的 `[TODO: ...]` 段：

- `description:` ← 步驟 1 收集到的「一句話描述」
- `## 任務` ← 步驟 1 收集到的「行為細節」，引用**步驟 3.5 已確認存在**的 tool 名稱與參數
- `## 輸出格式` ← 期望輸出形式

**禁止**在 skill body 內加任何「推送到 channel」「呼叫 send_http_request 給 Discord」「呼叫 MCP discord tool」之類的 notify 指令 —— scheduler 觸發後 runtime 自動把輸出送回原 caller channel（Discord 來源送回原頻道、CLI／HTTP 來源送回 action.log）。Skill body 只需專注產出**任務結果文字**。

### 5. 綁定時間

依步驟 2 結果呼叫，`skill_name` 用步驟 3 stdout 印出的完整 `<short>-<hash8>`：

```
add_task(time="<time_value>", skill_name="<short>-<hash8>")
# 或
add_cron(time="<cron_expression>", skill_name="<short>-<hash8>")
```

`skill_name` **不加 `scheduler-` 前綴**（內部會直查 `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md` 確認存在）。session_id 內部自動取 caller `e.SessionID`，不必傳。

成功會回 `ID: <hash>` 等資訊。失敗（skill 不存在、cron 表達式錯誤、`time` 已過）就 abort 本流程，向使用者回報原因。

### 6. 回報

簡短告知：

- skill 已建立: `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md`
- skill name: `<short>-<hash8>`（無前綴，hash 自動產生避免命名衝突）
- 排程: `add_task` / `add_cron` 的回應內容（含下次觸發時間、ID）

## 命名規則

| 項目 | 規則 | 範例 |
|---|---|---|
| short name（輸入 init script） | lowercase / hyphen-case，無 `scheduler-` 前綴、無 hash | `daily-hn-digest`、`tsmc-stock-watch` |
| hash suffix | init script 產生的 8-char hex random | `a3f9b2c1` |
| full name（檔案／frontmatter／add_task skill_name 用） | `<short>-<hash8>` | `tsmc-stock-watch-a3f9b2c1` |
| 目錄 | `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/` | `.../tsmc-stock-watch-a3f9b2c1/` |

**禁止**在任何環節加 `scheduler-` 前綴。`scheduler` 已表達於目錄路徑，加前綴只會造成 `scheduler/scheduler-foo-<hash>/` 之類的重複命名。

**禁止**自行產生／猜測 hash suffix。Hash **必須**由 init script 用 `secrets.token_hex(4)` 隨機生成，LLM 從 stdout 抓 `[OK] skill name:` 那行的值即可。

## 輸出路由（runtime 自動處理）

scheduler 觸發後，runtime 會把 subagent 產出的最終文字自動送回 caller 端：

| Caller session prefix | 路由行為 |
|---|---|
| `dc-*`（Discord） | 自動 `ChannelMessageSend` 回原頻道（含 ` - <skill 短名>` 標籤） |
| `cli-*`／`http-*`／TUI 觸發 | 留在 session history／action.log，由 caller 端工具讀取 |

**所以 skill body 不需要、也禁止**寫「推送到 channel」「呼叫 `send_http_request` 發 Discord webhook」「呼叫 MCP discord tool」之類的 notify 指令。寫了會在觸發時造成多餘的 token 與認證錯誤（subagent 沒 `DISCORD_BOT_TOKEN` 互動環境）。

## Secret／API Key（skill body 引用 token 時必看）

被觸發的 scheduler skill 跑在獨立 subagent，**不持有任何明文 secret**。若 body 內呼叫的 tool（如 `send_http_request`、自製 api_tool、script_tool）需要 API token：

- **命名格式**：`{品牌}_API_KEY`（SCREAMING_SNAKE_CASE），例 `OPENAI_API_KEY`、`CODEX_API_KEY`、`POLYGON_API_KEY`、`STAGING_API_KEY`
- **儲存位置**：macOS keychain 中 **service = `agenvoy`**、**account = key 名**，組合識別 `agenvoy.{key}`（例 `agenvoy.OPENAI_API_KEY`）
- **取值方式**：
  - api_tool：`auth.env: "<KEY_NAME>"`（schema 只記 key 名，無 `agenvoy.` 前綴）
  - script_tool：`GET http://localhost:17989/v1/key?key=<KEY_NAME>`（同樣不帶前綴）
  - skill body 純文字：直接引用 tool，**不**在 SKILL.md 寫明文 token、**不**寫 `export ENV=value` 之類指令
- **缺 key 處置**：若觸發時 keychain 無對應 key，subagent 會在 tool 端拿到 401／空值錯誤；skill body 不負責「補登」，請使用者預先用 `store_secret` 或 `/api-tool-add`／`/script-tool-add` 流程落地

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

**步驟 2** 正規化：`add_cron(time="*/5 * * * *", ...)`

**步驟 3** `run_command python3 scripts/init_scheduler_skill.py tsmc-stock-watch`

stdout：
```
[OK] created   : /Users/.../skills/scheduler/tsmc-stock-watch-a3f9b2c1/SKILL.md
[OK] skill name: tsmc-stock-watch-a3f9b2c1
...
```

抓出 full name `tsmc-stock-watch-a3f9b2c1`。

**步驟 4** `patch_file` 填入（frontmatter `name` 用 full name）：

```markdown
---
name: tsmc-stock-watch-a3f9b2c1
description: 每 5 分鐘抓取台積電 2330.TW 即時股價並提醒。
---

# Tsmc Stock Watch

## 任務

呼叫 `fetch_yahoo_finance` 取 `2330.TW` 的最新報價。

## 輸出格式

`台積電 2330.TW: NT$<price> (<change>% 從昨收)` 一行。
```

**步驟 5** `add_cron(time="*/5 * * * *", skill_name="tsmc-stock-watch-a3f9b2c1")`

**步驟 6** 回報：「已排程每 5 分鐘觸發 `tsmc-stock-watch-a3f9b2c1`。下次觸發 HH:MM。」

## 不做的事

- **不**用 `write_file` 直接建立 SKILL.md —— 必須走 `init_scheduler_skill.py`，避免結構錯誤（`<name>.md` vs `<name>/SKILL.md`）
- **不**在 short name、frontmatter、skill_name 任何位置加 `scheduler-` 前綴
- **不**留 `[TODO: ...]` 佔位符在最終 skill —— 步驟 4 須把所有 TODO 替換為具體內容
- **不**用任意預設值補齊時間 —— 缺時間就 `ask_user` 問清楚，不要「應該是 9 點」之類腦補
- **不**跳過步驟 5 的 `add_task` / `add_cron` —— skill 建立但沒綁時間 = 排程不會觸發
- **不**在 body 引用未經 `search_tools` 確認存在的 tool name —— 觸發時 subagent 找不到 tool 會直接 abort，使用者拿不到結果也看不到錯誤原因
