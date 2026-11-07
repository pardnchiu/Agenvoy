# Agenvoy - 技術文件

最後更新：2026-10-16

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.1 或更新版本
- macOS 或 Linux；Windows 請先安裝 WSL Linux 發行版，並在 WSL 終端機內執行 Agenvoy
- 至少一組模型供應商憑證；Telegram 與 Discord 需要各自的 bot token。語音轉文字、文字轉語音與圖片生成各需選定對應模型／provider 及其憑證

## 安裝

### 官方安裝程式

```bash
curl -fsSL https://agenvoy.com/scripts/install.sh | bash
agen
```

### Windows（透過 WSL）

請先以系統管理員身分開啟 PowerShell，查看並安裝一個 Linux 發行版：

```powershell
wsl --online --list
wsl --install <發行版名稱>
```

完成安裝、重新開機並進入 WSL 終端機後，再執行[官方安裝程式](#官方安裝程式)。

### 從原始碼建置

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -tags fts5 -ldflags "-X github.com/pardnchiu/agenvoy/internal/runtime.CurrentVersion=dev" -o agen ./cmd/app/
./agen
```

從原始碼建置需要 C 工具鏈：`mattn/go-sqlite3`（FTS5）沒有 pure-Go 路徑，必須 `CGO_ENABLED=1`。

### 使用 Makefile

```bash
make build
agen
```

`make build` 會將二進位檔安裝至 `/usr/local/bin/agen`（因此需要 `sudo`），並以 `extensions/` 的內容重建 `~/.config/agenvoy/skills/.system` 與 `~/.config/agenvoy/tools/.system`。

| Target                                  | 作用                                                                          |
| --------------------------------------- | ----------------------------------------------------------------------------- |
| `make build`                            | 建置（`HEAD` 剛好落在 tag 上時以該 tag 為版本，否則為 `(dev)`）、安裝到 `/usr/local/bin/agen`、更新內建 skill／工具 |
| `make app`                              | `stop` → `build` → 啟動 TUI                                                   |
| `make dev`                              | `stop` → `build` → 以 `--enable-claude-code` 啟動 TUI，`page/` 直接讀磁碟（`AGENVOY_PAGE_DIR`），前端改動免重建 |
| `make stop`                             | 停止 daemon                                                                   |
| `make update`                           | 執行官方更新程式                                                              |
| `make test`                             | `go test -v -count=1 ./...`                                                   |

### 直接執行

```bash
go run ./cmd/app/
```

## 設定

Agenvoy 使用 `~/.config/agenvoy/` 保存執行期資料，並將憑證存放於作業系統 keychain；不要將 API key 或 token 寫入專案檔案或 Git。

### 常用憑證

| Keychain 項目                                        | 用途                                               |
| ---------------------------------------------------- | -------------------------------------------------- |
| `OPENAI_API_KEY`                                     | OpenAI 與 OpenAI 音訊模型                          |
| `CLAUDE_API_KEY`、`GROK_API_KEY`、`DEEPSEEK_API_KEY`、`MISTRAL_API_KEY`、`NVIDIA_API_KEY` | 對應模型供應商 |
| `OPENROUTER_API_KEY` | OpenRouter 模型與 OpenRouter 音訊模型 |
| `TYPESAFE_API_KEY` | TypeSafe decision model；於 [TypeSafe Console](https://console.typesafe.ai/keys) 建立 |
| `CLOUDFLARE_API_KEY`、`CLOUDFLARE_ACCOUNT_ID` | Cloudflare（兩者皆必填；`CLOUDFLARE_GATEWAY_ID` 選填） |
| `TELEGRAM_TOKEN`、`DISCORD_TOKEN`                    | 聊天機器人整合                                     |
| `GEMINI_API_KEY`                                     | Gemini 模型供應商、Gemini 音訊模型與語音功能       |
| `OLLAMA-CLOUD_API_KEY`                               | Ollama Cloud（連字號是名稱的一部分）               |
| `COMPAT_<NAME>_API_KEY`                              | 名為 `<NAME>` 的本機／自訂 OpenAI 相容端點（選填） |

`copilot`、`codex` 與 `grok-oauth` 不使用 API key，而是從 `/model add` 發起 OAuth 登入；`claude-code` 改用本機 `claude` CLI，同樣不需 API key，且須以 `agen --enable-claude-code` 啟動（daemon 必須先停止）才會出現。

憑證依序從作業系統 keychain（service `agenvoy`）、Linux 的 `~/.config/agenvoy/.secrets`（`secret-tool` 不可用時的備援，權限 0600）與同名環境變數讀取，先找到者生效。

**Config › Model** 中，API key 類 provider 卡片附上 key 頁面連結（`Create one in the <provider> Console.`），OAuth 卡片附上訂閱頁連結（`Subscribe on the <provider> Plans page.`）。provider 模型清單，以及圖片、STT、TTS 模型清單都快取在 ToriiDB 15 分鐘。某 provider 的 key 儲存、OAuth 登入完成或清除時，會清掉它的快取清單。

### 音訊與圖片模型路由

語音轉文字（STT）、文字轉語音（TTS）與圖片生成模型，分別獨立於 session、dispatcher 與 summary 模型設定。在 TUI 中使用 `/model stt`、`/model tts` 與 `/model image` 設定；選擇列在最上方的 `disable` 可停用對應能力。開啟選單時 TUI 會先顯示 `loading...`，STT／TTS 可用模型會並行向已設定的 OpenAI、Gemini 與 OpenRouter provider 取得；圖片生成支援 OpenAI、Codex、Grok、Grok OAuth、Gemini 與 OpenRouter，沒有任何具憑證的 provider 時顯示 `no provider with credentials`。

自 **v0.34.4** 起，Telegram 與 Discord 暫停「收到語音輸入後自動產生並回傳語音輸出」的預設流程；本機 `generate_audio` 工具與音訊模型設定仍可使用，並可將產生的音訊檔傳送到任一頻道。

### 聊天機器人整合

Agenvoy 目前支援 Telegram 與 Discord。兩者都由本機 daemon 主動向外連線，因此主機不需要開放入站連接埠或公開端點；設定時只需要提供對應的 bot token；新對話之後須通過驗證碼（5 分鐘內有效），驗證碼會寫入 daemon log、顯示於 TUI，並在有設定時轉發到 `admin` 對話。其他聊天機器人平台除非能帶來明確的安全性改善，否則不在目前規劃內。

### Runtime 設定

主要設定檔是 `~/.config/agenvoy/config.json`。`limits` 欄位只能調高上限：每個值取「設定值」與「內建預設值」的較大者，低於預設的設定會被忽略；缺漏欄位直接採用預設值，不會寫回 `config.json`。

| 設定                                |    預設值 | 說明                                                                                                                                                                                    |
| ----------------------------------- | --------: | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `limits.max_tool_iterations`        |     `256` | 單次 Agent 工作的工具迭代上限                                                                                                                                                           |
| `limits.agent_send_timeout_seconds` |     `600` | 模型請求逾時秒數                                                                                                                                                                        |
| `limits.max_history_messages`       |      `16` | 保留的近期歷史訊息數                                                                                                                                                                    |
| `limits.max_history_bytes`          | `4194304` | 歷史訊息大小上限（位元組）                                                                                                                                                              |
| `reply_lang`                        |  `"auto"` | 回覆語言。`auto` 跟隨使用者訊息的語言；其他值為預設語言，不因使用者用什麼語言寫而改變；請求或 skill 指定其他語言時，該部分輸出改用指定的語言 |
| `output_dir`                        |      `""` | 請求沒指定位置時，替使用者產生的檔案放在哪：`write_result` 的輸出，以及 agent 預設要放到這裡的文件、匯出檔與圖片。空值為 `~/Downloads`，該資料夾不存在時為 `~/.config/agenvoy/download` |
| `model_tag`                         |      `{}` | 各模型的 tier，`{"<model>": "S"\|"A"\|"B"\|"C"\|"pass"}`；見[模型 tier](#模型-tier)                                                                                                     |
| `net_white_list`                    |      `[]` | 即使解析為 loopback 或私有位址，`http_request` 仍可連線的主機 |
| `decision_model`                    |      `""` | 供分派、監看檢查與內容過濾使用的 decision model，寫成 `typesafe@<model>` 或 `openrouter@<model>`；空字串或 `off` 為關閉。見 [Decision model](#decision-model) |
| `read_only_command`                 |      `[]` | 追加到內建唯讀白名單的指令，比對 `bin`、`bin subcommand` 或 `bin subcommand subcommand`（最長優先）；`run_command` 執行時不需確認 |

`reply_lang` 可填 `auto`、`configs/jsons/reply_lang.json` 內建的語言代碼（`en`、`zh-TW`、`zh-HK`、`zh-CN`、`ja`、`ko`、`es`、`fr`、`de`、`pt`、`it`、`ru`、`vi`、`th`、`id`、`ar`），或任意語言名稱（未列在內建清單時原字串交給模型）。`zh-TW` 與 `zh-HK` 分開：台灣與香港的繁體中文用詞與語法不同。作用範圍包含 agent system prompt、`/v1/chat/completions` system prompt、後續問題建議與 session 摘要。由 **Config › System** 設定會立即套用到執行中的 daemon；直接手改 `config.json` 則需重啟 daemon 才生效。

`output_dir` 儲存時會展開 `~` 並建立資料夾；建立不了的路徑會被拒絕，之後變得無法使用時則退回預設值。由 **Config › System** 設定會立即套用到執行中的 daemon。同一分頁會並列目前版本與最新 release，只有兩者不同時才顯示 **update** 按鈕，按下後開啟終端機執行 `agen update`。License 卡片附上 GitHub 原始碼連結。TUI 的 `/config`（回覆語言、輸出資料夾）寫入 `config.json` 的方式與手改相同。

套件內建預設值（目前不會從 `config.json` 讀取）：

| 常數                       | 預設值 | 說明                                                      |
| -------------------------- | -----: | --------------------------------------------------------- |
| `MAX_SESSION_TASKS`        |    `3` | 每個 session 的並行工作數上限；超出的任務排隊等待而非失敗 |
| `MAX_CONCURRENT_SUBAGENTS` |    `3` | 同時執行的 subagent leg 數，其餘排隊                      |
| `MAX_SUBAGENT_TIMEOUT_MIN` |   `30` | Subagent 逾時分鐘數                                       |
| `MAX_RESUME_WAIT_MIN`      |   `60` | Pending resume 等待回答的分鐘數                           |
| `MAX_CONCURRENT_TOOLS`     |    `5` | 單一模型回合內同時執行的 tool call 數，其餘排隊           |

```json
{
  "limits": {
    "max_tool_iterations": 512,
    "agent_send_timeout_seconds": 900
  }
}
```

範例值須高於內建預設值才會生效。

### TUI 執行模式

目前 runtime 內建 13 個模型供應商（含 Ollama Cloud），以 `--enable-claude-code` 啟動時另有 `claude-code`；另有 `compat` 項目可接本機或自訂的 OpenAI 相容端點（Ollama、LM Studio、自架 gateway）。

`/model add` 會以 `GET /models` 偵測本機的 Ollama（`http://localhost:11434/v1`）與 llama.cpp（`http://localhost:8080/v1`），有回應的會以 **Ollama Local**／**Llama.cpp Local** 附加在 `/model add` 的 **Custom** 分頁末端（偵測逾時 500ms），選取後直接進入模型挑選，不再詢問網址或 key。這兩個端點是內建的（`configs/jsons/local_compat.json`），不會寫入任何設定。其他 port 或主機走 **Local/Custom** 新增，網址會記錄到 `config.json` 的 `compats`。兩種情況的模型清單都由端點自己的 `GET /models` 取得。端點模型以 `<name>@<model>` 註冊，端點名稱轉小寫（`ollama@gemma3:4b`）；舊的 `compat[NAME]@<model>` 寫法仍可使用，`config.json` 每次載入或儲存時會改寫成新格式。

compat 通道只送 request body 與 `Authorization: Bearer <key>`，不夾帶任何廠商私有 header，因此要求私有 header 的端點不在支援範圍。收錄範圍限三類：自營模型的供應商、相容 OpenAI API 的 NIM，以及 Cloudflare。opencode Go（`opencode.ai/zen/go/v1`）依此排除——自 [2026-09-03 公告](https://x.com/opencode/status/2095410501400289576)起，未帶依對話變動的 `x-opencode-session` 會被拒絕（`400 MissingSessionID`）；Chat Completions 本身無狀態，要求 client 依對話追蹤並輪替廠商私有 header 已非 OpenAI 相容行為。

當輸入區為空時，按下 `Shift+F` 可切換 fast mode。啟用時，標題列會顯示 `[fast]`。Fast mode 只存在於目前行程，不會保存至 `config.json`；它會透過 `go-llm-router` 傳遞 `core.ModeFast`，讓支援的 provider backend 要求更快速的服務層級。關閉 fast mode 時則使用預設模式。

### Agent 選擇與確認路由

請求以 `/<skill_name>` 開頭或符合 Skill 時，dispatcher 會收到該 Skill 的說明作為選擇提示；排程觸發的 Skill 走 subagent 路徑，以 Skill 內容作為請求、不帶此提示，模型在對話中以 `run_skill` 載入的 Skill 也不會重新選模型。因此模型選擇會反映目前任務契約，而不只依賴使用者輸入文字。建立 prompt 時，Agenvoy 會加入共用官方操作指南，並在有設定時加入符合目前所選模型的專屬指南。

### 模型 tier

每個已註冊模型都可以在 `model_tag` 設定 tier：

| Tier   | 意義                                                                      |
| ------ | ------------------------------------------------------------------------- |
| `S`    | 最強；寫程式，或明確要求深度、精確的工作                                  |
| `A`    | 大部分工作的預設，比旗艦低一階                                            |
| `B`    | 主流中階                                                                  |
| `C`    | 快又便宜，能照指示穩定呼叫工具                                            |
| `pass` | 不被自動分派、subagent 與 fallback 選中，請求點名也不選；只有指定給某個 session 時使用 |

在 TUI 的 `/model` 對模型列按 `t`，或在 **Config › Model › Fallback Priority** 每張卡片的 tier 按鈕設定。tier 每次請求都重新讀取，改完不需重啟。

session 固定了模型就不排序也不 fallback；但 session reasoning 為 `auto` 時仍會呼叫 selector 取得 reasoning 等級。否則 dispatcher 會收到每個已註冊模型解析後的 tier；沒設 tier 的模型依名稱判斷（`claude-opus` 為 S、`claude-sonnet` 為 A、`claude-haiku` 為 B、`*-mini` 與 `gemma*`／`qwen*` 等開源模型為 C 等）。排序依工作類型決定：寫程式或明確要求深度、精確 → S 優先；從多個來源蒐集再綜整的研究 → S 優先；審閱、規劃、撰寫與其餘工作 → A 優先；打招呼、短答、閒聊、翻譯 → B 優先；用工具取資料並原樣回傳 → C 優先。LLM dispatcher、decision model 與 subagent planner 共用同一張工作類型表與同一套名稱規則，排序結果一致。同一 tier 內，訂閱制 provider `claude-code`、其次 `codex` 的模型排在其他 provider 之前。同一個模型註冊在多個 provider 時，順序為 `claude-code`、`codex`／`grok-oauth`、`copilot`、直接 API、`openrouter`（比對時 `openrouter` 的模型名去掉 vendor 前綴）；點名或延續的模型維持在最前面，只重排其後的候選。`copilot` 轉售的 context window 較小，長輸入或 `research` 類工作會優先選其他 provider。`pass` 同時由 prompt 與 fallback 規範：dispatcher 被要求一律不回傳 `pass` 模型，selector 回傳的 `pass` 名稱也會被剔除；fallback 依優先順序嘗試其他已註冊模型（排除 `pass`），跳過失敗模型的 provider，並逐一以健康檢查（10 秒）確認可用；整輪皆失敗時最多重試 4 輪才失敗。串流沒有回應時，watchdog 也會切換到下一個模型。context 上限取自 `llm-io.agenvoy.com`，快取 1 小時；`copilot@` 模型會非同步向 GitHub Copilot models API 暖機專用模型限制快取，最多每 24 小時刷新一次；若 chat 模型有回報 `max_context_window_tokens`，就採用該值。未知 Copilot 模型與其他未列模型使用一般 128K 輸入上限。TUI 顯示目前 context token 相對於解析出的輸入上限；歷史達上限 80% 時開始 compact。

主 agent 一旦分派，只負責拆分任務、呼叫 leg 與綜整結果；每條 leg 只做一種工作——collect、analyze、compare、review、transform 或 code。`reasoning_guide` 的 `subagent_dispatch` 主題會把同一套 tier 整體降一階（S→A、A→B、B→C）提供給主 agent 參考：collect → fetch（C>B>A>S）、transform → chat（C>B>A>S）、analyze／compare／review → work（B>A>C>S）、程式碼或高精確工作 → code（A>B>C>S）；這是給主 agent 的選模建議，程式本身不強制。每條 leg 的 `model` 為必填，以即時的 registry 驗證，啟動後新增的模型也能使用，未註冊或 `pass` 模型會被拒絕。明確指定的模型會覆蓋 `self_id` session 自己固定的模型；傳 `auto` 時先沿用該 session 固定的模型，沒有才由 dispatcher 依 leg 的任務選模。`reasoning` 預設 `auto`（沿用 session 設定，仍為 `auto` 時由 selector 決定），不合法的值直接回傳錯誤並列出可用等級。最多同時執行 3 條 leg，報告寫入 `~/.config/agenvoy/download/temp-<report_name>.md`。

### Decision model

decision model 以不到一秒的時間回答型別化的問題（從選項中挑一個並附信心值），Agenvoy 用它分派請求，並過濾送進主模型的內容。提供者有兩個：`typesafe`（`TYPESAFE_API_KEY`）與 `openrouter`（`OPENROUTER_API_KEY`，輸出為 decisions 的模型；`cloudflare/clairic`、`respan/span-01*` 與 `scaledown/classify-1` 不接受 Agenvoy 的請求格式，因此不列入）。可選清單快取 15 分鐘，provider key 變更時清除。在 TUI 以 `/model decision`、或在 **Config › Model** 的 **Decision** 卡片選擇模型；設定存於 `decision_model`，`off`（或空字串）會關閉下列所有用途。TypeSafe key 的新增方式與其他 API key 相同：TUI 為 `/model add` → **API Key** → **TypeSafe**，Web 為 **API Key** 分頁的 TypeSafe 卡片；存檔只會儲存 key，模型再到 Decision 選擇。TypeSafe 的模型也不會出現在 OpenRouter 的聊天模型清單中。

每次呼叫逾時 3 秒。除了分派與 skill 清單，回答的信心值須達 0.7 才會採用；呼叫失敗時，行為與沒有 decision model 時相同。

| 用途 | 判斷內容 | 沒有回答時 |
| --- | --- | --- |
| 分派 | 工作類型、是否明確點名模型、是否延續上一則請求；與 dispatcher 模型同時執行，先取得有效回答者勝出 | 採用 dispatcher 模型的回答 |
| 監看（`background_command`） | 命中錯誤關鍵字的輸出是否真的是失敗，以及是否只是重複上次的回報；誤判與重複都不回報 | 照常回報該段輸出並註明檢查失敗。未設定 decision model 時，每次命中關鍵字都會回報：等輸出穩定（靜止 2 秒，最多等 10 秒）、每 30 秒最多一次，附最新至多 8 KiB 輸出 |
| `search_web` | 哪些結果標題與請求相關，其餘剔除 | 保留全部結果 |
| `fetch_page`（markdown） | 哪些區塊是頁面雜訊，存快取前移除；頁面是否與請求相關，不相關則不交給模型 | 回傳完整頁面 |
| Turn context | 現有工具是否涵蓋這個請求，否則提示查閱建工具指南；哪些 skill 相關，其餘不列入清單 | 不提示；列出全部 skill |
| Session 摘要 | 哪些對話是雜訊、哪些 `key_decisions`／`past_discussions` 已過時，合併前一併移除 | 保留全部對話與條目 |
| Context 超長 | 哪些工具步驟對請求已不需要，在依最舊優先刪減前先移除；最新一步與 `write_todo` 一律保留 | 只做最舊優先刪減 |

只要設定了 `decision_model`，分派就會使用 decision model；它與 dispatcher 模型同時執行，先取得有效回答者勝出。分派時 decision model 會收到請求內容與最近至多四則 user／assistant 訊息，每則截至 2,048 字元。它會把請求歸成一種工作類型，模型排序與 reasoning 等級都依這個結果。另一題判斷請求是否明確指定「使用／用 <模型>」（不含 `pass` 模型），是的話把該模型排第一。session 的前一個模型仍在候選內時，第三題判斷請求是否延續上一則使用者訊息；是的話把該模型排在點名模型之後，且除非點名了其他模型，reasoning 等級沿用前一次的等級（取代本次分類結果）。前一個模型在每次成功回覆後記錄於 ToriiDB db_0 的 `lastModel:<session>`，`openai`／`codex` 保留 30 分鐘、`gemini` 60 分鐘、其他 provider 5 分鐘。

| 工作類型 | 涵蓋 | tier 順序 | reasoning |
| --- | --- | --- | --- |
| `code` | 寫、修、除錯、測試程式；明確要求深度或精確 | S>A>B>C | `xhigh` |
| `chat` | 打招呼、短答、翻譯 | B>C>A>S | `none` |
| `fetch` | 用工具取資料並原樣回傳，不做判斷 | C>B>A>S | `low` |
| `research` | 從多個來源蒐集並下結論：研究、分析、比較、報告 | S>A>B>C | `high` |
| `work` | 規劃、review、起草或整理使用者手上的內容，及其他 | A>S>B>C | `medium` |

tier 取自 `model_tag`，沒設定的依模型名稱判斷；`pass` 模型不參與此排序，也不進入 fallback，請求點名也不選；只有指定給某個 session 時才會使用。預設 dispatcher 的 prompt 使用同樣五種工作類型與 tier 順序：第一行回工作類型、第二行回模型名單，因此不需要 decision model 也能取得上表的 reasoning 等級。三件事各自獨立：**用哪個 selector**（已設定 decision model → decision model 與 dispatcher 模型同時執行，否則只用 dispatcher 模型）、**用哪個模型**（session 模型為 `auto` 時才採用 selector 的排序，指定模型則維持）、**用哪個 reasoning 等級**（session reasoning 為 `auto` 時才採用 selector 的等級，selector 沒給時退回 `medium`，其他等級維持）。請求自帶的等級（例如 `reasoning_effort`）仍優先於兩者。新 session（含 `temp-`）預設 reasoning 為 `medium`，需改為 `auto` 才會採用 selector 的等級。

TUI、Web、Telegram 與 Discord 的回覆 footer 以 `model/reasoning` 顯示實際使用的等級。`action.log` 的 `done` 行會記錄 `reasoning=<level>`，Web 聊天重新整理後仍會顯示。Web 的 reasoning 紀錄開頭為 `Model(<model> <level>)`，之後換模型時加上 `Switch(...)`；已完成的那一輪重新整理後不再顯示這些行。

互動請求也會攜帶來源前綴。CLI 確認僅由 TUI 接收，Web 請求由 Web 確認串流處理，Telegram 與 Discord 則由各自對應的頻道 listener 接收。非 TUI 確認會在五分鐘後逾時，該任務取消並保留為 pending，可稍後恢復，避免某個頻道攔截或長期占用其他頻道的提示。

各介面的選項一致：**Yes**、**Yes, don't ask again**、**Yes, allow this turn**、**No, skip this**（可附選填理由回給 agent，agent 會收到「使用者拒絕了這次呼叫，繼續下一步」）、**Abort task**（停止任務並保留 pending 供接續）與 **Cancel task**（停止任務並刪除 pending）。`run_command` 與 `background_command` 的確認另附一句由 dispatcher model 寫的說明：只在實際跳出確認時呼叫一次（唯讀指令不會詢問），只帶請求與該次呼叫，不寫入 session 歷史；未設定 dispatcher model 或 10 秒內未取得回應時不顯示。

### 受限路徑與受限指令

`$HOME` 以外的路徑與敏感清單內的路徑（`configs/jsons/sensitive_path.json`，可由 `config.json` 的 `sensitive_path` 擴充）不會直接被拒絕：`boundary.Restricted` 會收集它們並發出確認，且該確認同時要求作業系統密碼（TUI 與 Web 確認框皆為密碼欄，由 daemon 驗證）；`run_command`、`background_command` 以 argv `['sudo', ...]` 或 `sandbox: false` 執行時也走同一道關卡，任何權限模式都一樣；驗證通過後 `sudo` 以 `sudo -n` 沿用驗證留下的授權執行。TUI 的 `!` shell 指令含 `sudo` 且沒有有效授權時，會先跳出密碼框。核准綁定該 session 與該路徑，於 daemon 存續期間有效；密碼欄是否出現依 sudo 自己的 timestamp（`sudo -n -v`），該 ticket 仍有效時彈窗不會再出現密碼欄。密碼錯誤時彈窗保留並顯示錯誤，可重新輸入，驗證成功才關閉。受限確認不提供 **Yes, don't ask again** 與 **Yes, allow this turn**。`config.json` 的 `denied_path` 與 `denied_command` 為硬拒，任何核准都解不開；不在 `denied_command` 的指令一律可執行，但仍會經過 `run_command` 的一般確認。

自動化之前要知道的事項：

- 一般工具確認會回到原始 CLI、Web、Telegram 或 Discord 頻道；TUI 只監聽 CLI 來源的請求。
- Telegram 與 Discord 無法收集系統密碼，受限確認不提供 **Yes**，並引導至本機的 TUI 或 Web UI。Web UI 只接受本機來源的核准，否則保留確認卡片並顯示錯誤。
- `run_command` 的沙箱只約束寫入不約束讀取，因此指令只有在作業系統本身拒絕時才會在某個路徑上失敗；檔案工具（`read_files`、`find_files`、`open_file`、`file_history`）讀取 `$HOME` 以外或敏感路徑時仍需確認，`denied_path` 讀寫皆拒。

`$HOME` 一律可寫，不需要任何設定。指令要寫到 `$HOME` 以外時，agent 會在**那一次呼叫**自己附上 `write_paths`，你會看到確認框並輸入系統密碼，核准後那幾個路徑才額外綁進沙箱。

### MCP Client

MCP client 與 server 位於 `internal/runtime/mcp`，並使用官方 [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)。在 `~/.config/agenvoy/mcp.json` 登錄 stdio 或 streamable HTTP MCP server。Client 會訂閱工具清單變更通知，並每 60 秒輪詢一次（單次逾時 30 秒），遠端 server 更新目錄時會重新註冊工具；`headers` 的 `Authorization` 值沒有 scheme 時會自動補上 `Bearer `；server instructions 會注入 agent system prompt；工具結果中的圖片會以 data URL（`image_0`、`image_1`...）連同文字交給 agent。server 項目另可設定 `env`（stdio）、`auth: "oauth"` 與 `oauth_client_name`（HTTP）；`args`、`env` 與 `headers` 內的 `${VAR}` 會展開：

```json
{
  "servers": {
    "local-tools": {
      "command": "node",
      "args": ["/absolute/path/server.js"]
    },
    "remote-tools": {
      "url": "http://127.0.0.1:8000/mcp",
      "headers": { "Authorization": "Bearer ${MCP_TOKEN}" }
    }
  }
}
```

Agenvoy 本身也是 MCP server，走 stdio：`agen` 的 stdin 不是 TTY（有管線輸入）時就啟動 MCP server 而非 TUI。要掛進其他 agent 請自行寫入該 agent 的設定檔——Claude Code 用 `~/.claude.json`、OpenCode 用 `~/.config/opencode/opencode.jsonc`、Codex 用 `~/.codex/config.toml`：

```json
{
  "mcpServers": {
    "agenvoy": { "command": "agen" }
  }
}
```

```toml
[mcp_servers.agenvoy]
command = "agen"
```

### Session 分類

TUI 的 `/session` 選擇器會依 ID 前綴分類：`cli-` 代表本機 CLI、`tg-` 代表 Telegram、`dc-` 代表 Discord、`chat-` 代表 Web／API；`temp-`（短期工作）的 session 不會列出。偵測到至少兩個群組時，選擇器會顯示 `all` 與各前綴分頁，並將目前 session 排在最前。Daemon 會以 `fsnotify` 監看新建立的 session 目錄，將 session ID 與設定名稱寫入 daemon log。

Session persona 現存於 history SQLite 資料庫。`self_id` 會正規化為小寫，只接受最多 32 個 ASCII 字母、數字、`_` 或 `-`，非空值必須唯一。舊版每個 session 的 `bot.json`、`status.json` 等檔案自 v1.1.2 起不再自動遷移。

## 使用方式

### 啟動 TUI

```bash
agen
```

直接輸入文字即在當前 session 執行；其餘皆為斜線指令，只打 `/` 會開啟選單，其中也包含已安裝的 skill 與排程項目。

| 指令                       | 用途                                                                                                                                                                                                                                                                                                |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/model`                   | 八個分頁，每個路由角色一個：**model** 挑選 session 模型（`auto` 或已註冊模型；`d` 移除游標所在模型、`t` 設定其 tier、`w`／`s` 上下移動調整 fallback 順序，Enter 時儲存）與 `add` 新增 provider（`o` 開啟 provider 主控台），**reasoning** 挑選該 session 的等級（`auto` 或固定等級），**dispatch** 挑選路由模型或 `Decision Model`，**summary** 挑選摘要模型（`auto` 清除），**decision** 挑選 decision model（`disable` 關閉），**image**／**stt**／**tts** 各自挑選該角色的模型或 provider（`disable` 清除）。`/model <分頁名>` 或 `/model add` 可直接開啟對應畫面 |
| `/mcp`                     | 列出 MCP server（`d` 移除）並 `add` 新增；單一 server 可登入、設定 OAuth client、以 `tools` 多選設定免確認工具（第一列為 `all`）、重連                                                                                                                                                              |
| `/session` `/new`          | 以 self id 切換 session（`d` 刪除）或建立新的；清單橫跨兩種以上前綴時依 `all`／`cli-`／`tg-`／`dc-`／`chat-`／`temp-` 分頁；`/session` 清單下方可編輯當前 session 的 `name`／`self id`／`role`（`/session name\|id\|role` 直接開啟） |
| `/compact` `/reset`        | 同一個彈窗含三個分頁：`summary` 唯讀檢視目前滾動摘要；`compact` 以 LLM 分析移除冗餘對話；`reset` 重設 session（需二次確認）：`summary` 先重建摘要並保留，`all` 連摘要一併清除。`/compact` 開在 compact 分頁、`/reset` 開在 reset 分頁 |
| `/skill`                   | 三個 tab。`permission`：Enter 切換目前這個 skill 的 `always allow`／`ask`，`always allow` 為全域一律允許（略過權限確認），`o` 開啟該 skill 所在資料夾；`system`：多選官方 skill，勾選將 `github.com/agenvoy/skill-<name>` clone 至 `~/.config/agenvoy/skills/.system_design/<name>`，取消勾選則刪除；`source`：多選要讀取的外部 skill 來源（`claude`、`codex`、`copilot`、`opencode`、`agents`，即 `~/.<source>/skills`），取消勾選則略過 |
| `/role`                    | 列出、新增或編輯 role（標題＋描述）                                                                                                                                                                                                                                                                 |
| `/channel`                 | 兩個分頁 `telegram`／`discord`：啟用（token 先驗證再存入）或停用該頻道，`d` 撤銷已授權對話                                                                                                                                                                                                          |
| `/config`                  | 可搜尋的設定清單，`enter` 修改游標所在項目：登入時自動啟動 daemon（macOS 走 launchd agent，Linux 走 systemd user unit）、回覆語言（`auto` 跟隨每則訊息）、輸出資料夾（產生的檔案存放位置；留空為 `~/Downloads`）、channel admin（接收新對話驗證碼的已授權對話；留空則只寫 log）與 official guide（是否注入官方模型指引） |
| `/schedule`                | 週期（cron）與單次（task）排程合併為一個清單；`enter` 立即執行、`d` 刪除；新增或編輯請直接交代 agent                                                                                                                                                                                                |
| `/pending`                 | 列出並恢復中斷的任務（`ask_user`、錯誤復原）；`d` 經確認後移除                                                                                                                                                                                                                                      |
| `/resume` `/log` `/usage`  | 重載可見對話、以 `$PAGER` 追蹤 `daemon.log`、查看各模型 token 用量（上方為本 session、下方為全部 session，24h／7d／28d）                                                                                                                                                                            |
| `/key`                     | 編輯已儲存的憑證（`d` 刪除）                                                                                                                                                                                                                                                                        |
| `/update`                  | 確認後停止 daemon、執行 `agen update`（下載最新預先建置的 release）並離開                                                                                                                                                                                                                           |
| `/clear` `/exit` `/quit`   | 清除可見對話，或離開 TUI（`/quit` 為 `/exit` 的別名；daemon 繼續執行）                                                                                                                                                                                                                              |
| `/<skill>` `/sched-<name>` | 直接執行已安裝的 skill 或排程項目                                                                                                                                                                                                                                                                   |

輸入區為空時可用的快捷鍵：

| 按鍵                  | 動作                        |
| --------------------- | --------------------------- |
| `Shift+W` / `Shift+S` | 反向／正向切換 session 模型 |
| `Shift+A` / `Shift+D` | 切換 reasoning 等級（含 `auto`） |
| `Shift+F`             | 切換 fast mode              |
| `Shift+U`             | 查看 provider 額度與餘額    |
| `Shift+G`             | 切換是否載入工作目錄的 `CLAUDE.md`／`AGENTS.md` 專案指引；開啟時狀態列顯示檔名 |
| `Shift+Tab`           | 切換 allow-all（略過工具確認）；只要沒有任務在執行且未開啟指令選單即可使用 |

在彈出視窗中，`esc` 會回到開啟它的上一頁，只有第一頁才會關閉；確認、密碼與 `ask_user` 視窗不理會 `esc`，須選擇選項；`/usage` 這類純列表視窗以 ↑／↓ 捲動而非選取。

任務執行中按 `esc`：尚未輸出任何內容時直接取消，否則詢問 **No**／**Yes cancel**／**Pause keep pending**。取消（含 `ctrl+c`、確認框中的 **Cancel task**，以及 `POST /v1/session/:id/cancel/:task_hash`）會一併移除該任務的 pending；暫停、確認框中的 **Abort task** 與其他中斷（逾時、視窗關閉等）只停止執行並保留 pending，可從 `/pending` 恢復。任務執行中只能送一般文字作為 steer，`/` 指令不會列出也不會送出。非彈窗狀態下 `esc` 依序為：先關閉 `/` 指令選單，再關閉 notice 區塊，再取消任務，最後清空輸入。狀態列顯示 `Connecting` 時，一般訊息、Skill、`/pending` 與 cron 派發都不會送出；本機 `/` 指令照常可用。

### 管理 daemon

```bash
agen stop
agen update
```

執行 `agen` 時，若本機 daemon 尚未執行會自動啟動。

### stdin MCP Server

當 stdin 不是終端機時，Agenvoy 會啟動 newline-delimited JSON-RPC MCP server：

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | agen
```

支援 `initialize`、`notifications/initialized`、`notifications/cancelled`、`tools/list`、`tools/call` 與 `ping`。

### Web 與檔案回應顯示

後端會在 result、SSE、pending 與 multilog handler 間保留 `[SEND_FILE:...]` 標記，讓頻道 consumer 仍可取得檔案傳遞 metadata。Web dashboard 只在顯示訊息文字時移除標記，因此使用者看到的是回應內容，不會看到內部傳輸標記。

Web 對話以豐富格式渲染回覆：```` ```chartjs ```` 區塊顯示為圖表（bar、line、pie、doughnut、radar、polarArea、scatter、bubble），```` ```mermaid ```` 區塊顯示為圖（flowchart、sequenceDiagram、stateDiagram-v2、classDiagram、erDiagram、gantt、timeline），```` ```math ```` 區塊與行內 `$...$` 以 KaTeX 顯示數學公式，其餘程式碼區塊套用語法高亮；無法解析的區塊改顯示原始碼。Telegram 與 Discord 只收到純 markdown。

### HTTP API

Daemon 監聽 `127.0.0.1:17989` 與 `[::1]:17989`，連接埠固定不可調整：

```bash
curl --fail-with-body -sS \
  -H 'Content-Type: application/json' \
  -d '{"content":"列出目前可用工具","persist":false}' \
  http://127.0.0.1:17989/v1/send
```

`/v1/chat/completions` 為 OpenAI 相容的 stateless endpoint；需在每次請求中帶入延續對話所需的 `messages`。`reasoning_effort` 接受 `none` `low` `medium` `high` `xhigh` `max`（另支援別名 `minimal` `extra` `ultra`）；未帶或無法識別的值採用 session 等級；session 設為 `auto` 時採用 selector 的等級，否則退回 `medium`。

## 命令列參考

| 指令   | 語法                   | 說明                               |
| ------ | ---------------------- | ---------------------------------- |
| TUI    | `agen`                 | 開啟或連接本機 daemon 的互動式 TUI |
| 停止   | `agen stop`            | 停止 daemon                        |
| 更新   | `agen update`          | 執行官方更新腳本                   |
| Daemon | `agen --daemon [--enable-claude-code]` | 直接啟動 daemon（可附帶啟用 claude-code） |
| Claude Code | `agen --enable-claude-code` | 啟用 claude-code provider 並開啟 TUI；daemon 須先以 `agen stop` 停止 |
| MCP    | 非 TTY stdin 的 `agen` | 從 stdin 提供 MCP JSON-RPC         |

## HTTP API 參考

Daemon 只綁定 loopback（`127.0.0.1` 與 `[::1]`）。標示 **local** 的 endpoint 另外要求請求來源必須是 `127.0.0.1`／`::1`（`localhostOnly()` 守衛）——這些會動到 credential、設定檔或行程生命週期，設計上是給同機器的 dashboard 用，不是給遠端 client 呼叫。

**Agent 執行**

| Method | Path                   | 說明                                                                                                                                                                                                                                                                                         |
| ------ | ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST` | `/v1/send`             | 執行 Agent：`{content, session_id?, sse?, persist?, model?, work_dir?, skill?, system_prompt?, exclude_tools?}`。未帶 `session_id` 時建立 `temp-`（`persist:true` 則 `chat-`）session；該 session 已在執行時改為 steer，回 `{session_id, steer:true}`；`content` 為空回 400 |
| `POST` | `/v1/chat/completions` | OpenAI 相容且 stateless 的 chat completions                                                                                                                                                                                                                                                  |
| `GET`  | `/v1/info/version`     | 編譯時寫入的版本（`{version, dev}`）；未帶 tag 的建置 `dev` 為 true                                                                                                                                                                                                                          |
| `GET`  | `/v1/log`              | SSE：不帶參數時只送 daemon `slog`（`EventDaemonLog`，`source` 為層級），與 TUI 標題列同一份來源；內含新對話驗證碼，故 daemon frame 僅對 loopback 來源附加。`?sessions=a,b` 於同一條連線加上該些 session 的事件，`replay=0` 略過回放，`daemon=0` 去掉 daemon frame。遠端來源必須帶 `sessions` |
| `GET`  | `/v1/voice/live`       | **local** — WebSocket，轉接 Gemini Live 即時語音（需 `GEMINI_API_KEY`，缺少時以 close code 4401 關閉）；模型可透過 `send_to_chat`、`read_history`、`read_task` 與主對話互動 |
| `GET`  | `/v1/mcp/tools`        | 列出已連線 MCP server 註冊的工具（`mcp__*`）                                                                                                                                                                                                                                                 |

**模型**

| Method          | Path                            | 說明                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --------------- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET`           | `/v1/models`                    | 列出已註冊模型（OpenAI `{object:"list", data:[...]}` 格式，另附 `models` 名稱陣列，含 `auto`） |
| `GET`           | `/v1/models/*id`                | 讀取單一已註冊模型                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `POST` `DELETE` | `/v1/models` `/v1/models/*name` | **local** — `POST` 收 `{prefix, models}`，以 `models` 整組取代該 prefix 底下的已註冊模型（未列出的會被移除），回 `{ok, added, removed}`；`prefix` 須為 `GET /v1/providers` 的 provider id，或本機／自訂端點的名稱（`ollama`、`llama.cpp`，或經 `POST /v1/provider/compat/key` 記錄的名稱）。`DELETE` 移除單一模型，不存在回 404，使用中的 session 重設為 `auto` |
| `GET` `POST`    | `/v1/model`                     | **local** — 讀取或設定模型路由：`dispatcher`、`summary`、`image`、`decision`、`stt`、`tts`；讀取時另回傳 `image_options`、`image_providers`、`decision_options`、`audio_providers`。`dispatcher` 與 `summary` 使用已註冊模型名稱（`prefix@model`）；`image` 使用 provider 名稱（`openai`、`codex`、`grok`、`grok-oauth`、`gemini`、`openrouter`）；`decision` 必須是 `decision_options` 中的模型；`stt`、`tts` 必須是 `GET /v1/model/audio` 回傳的可用選項。`POST` 為部分更新，未帶或 `null` 不變，空字串清除設定，`off` 僅可作為清除 `image` 的別名。無效模型、provider 或音訊選項會被拒絕，且不會寫入。`dispatcher_beta` 為布林值；尚未設定 decision model 卻設為 `true` 時回 400。 |
| `GET`           | `/v1/model/audio`               | **local** — 列出由已設定 OpenAI、Gemini 與 OpenRouter provider 取得的 `stt_options`、`tts_options`；各清單快取 15 分鐘。                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `GET` `POST`    | `/v1/model/priority`            | **local** — 讀取／調整已註冊模型的順序。順序決定 fallback 優先度，選中的模型失敗後，其餘模型由上而下依序嘗試（排除 `pass`），最後一個非 `pass` 模型為最後防線。`GET` 另回傳 `tiers`（`{model: tier}`）與 `tier_options`（`[{tier, detail}]`，空 tier 排在第一個）。`POST` `{models}` 依序把列出的名稱移到最前面，其餘接在後面；未知名稱回 400                                                                                                                                                                                            |
| `POST`          | `/v1/model/tier`                | **local** — `{model, tier}` 設定單一模型的 tier（`S` `A` `B` `C` `pass`）；`""` 清除。未註冊的模型或未知 tier 回 400                                                                                                                                                                                                                                                                                                                                                                                         |

**Session**

| Method                | Path                                        | 說明                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| --------------------- | ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`                 | `/v1/sessions`                              | 列出 session 與狀態；`?filter=all\|active\|permanent\|temporary` 篩選 |
| `GET`                 | `/v1/usage`                                 | **local** — 所有 session 在 24h／7d／28d 的 token 用量合計；每個模型另帶 `elapsed_ms`（模型送出耗時合計）與 `output_tps`（每秒輸出 token，僅計有量到耗時的紀錄）                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `POST`                | `/v1/session`                               | **local** — 建立 session，`{prefix}` 預設 `cli-`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `GET` `POST` `DELETE` | `/v1/session/:id`                           | **local** — 單一 session 的完整狀態：`id`／`self_id`／`name`／`role`（同時以 `rule` 回傳供舊客戶端使用）／`state`／`model`／`reasoning`／`levels`／`count`。`POST` 為部分更新，`self_id`／`name`／`role`（仍接受舊的 `rule`）／`model`／`reasoning` 皆選填，未帶（或 `null`）的欄位不動；`model: ""` 重設為 `auto`，`reasoning` 須為 `levels` 之一（`levels` 第一項為 `auto`）。`GET` 與 `POST` 回傳同一種物件，`self_id` 重複回 409。`DELETE` 移除 session 目錄、歷史、狀態與向量。`GET` 另接受 `?chat=1` 附上原始 action log（放在 `chat`）與 `?usage=1` 附上 24h/7d/28d 各模型 token 用量（放在 `usage`，與 TUI `/usage` 畫面同一套聚合邏輯，含 `elapsed_ms` 與 `output_tps`）；兩者預設關閉，因為 log 可能很大 |
| `GET`                 | `/v1/session/:id/task`                      | 列出可恢復的待完成（`ask_user`／confirm）工作；仍在執行中的不列入——執行期間每 55 秒刷新 ToriiDB 的 `action:<session_id>:<task_hash>`（TTL 60 秒），視窗關閉或程序被砍的任務一分鐘內會重新出現                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `GET`                 | `/v1/session/:id/task/:task_hash/questions` | 取得待完成工作的問題內容                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `POST`                | `/v1/session/:id/task/:task_hash/resume`    | 回答待完成工作並恢復執行                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `DELETE`              | `/v1/session/:id/task/:task_hash`           | 直接捨棄待完成工作，不回答                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `POST`                | `/v1/session/:id/cancel/:task_hash`         | 以使用者取消的方式取消單一執行中的任務，並一併移除其 pending，回 `{ok, cancelled:true}`；該 hash 不在本行程執行中時仍回 200 `{ok, cancelled:false, stale:true}`，並向該 session 發布取消事件 |
| `POST`                | `/v1/session/:id/confirm/:confirm_hash`     | 回覆等待中的工具確認：`{approve, remember?, allow_turn?, abort?, cancel?, reason?, password?}`：`abort` 停止任務並保留 pending，`cancel` 停止任務並刪除 pending，`approve: false` 時 `reason` 會回給 agent。核准受限路徑、`sudo` 指令或 `sandbox: false` 指令須帶 `password`，且只接受本機來源（否則 403），系統密碼錯誤回 401；確認已處理或逾時回 410                                                                                                                                                                                                                                                                                                                                                                                                              |
| `POST`                | `/v1/session/:id/memory`                    | **local** — 對該 session 執行一項記憶操作，由 `action` 決定：`summary` 重建滾動摘要並回 `count`；`compact` 丟掉較舊的訊息並回 `removed`；`reset` 清空對話並回 `removed`，且必須帶 `mode`——`summary` 保留滾動摘要（摘要無法產生時中止，避免遺失 context），`all` 連摘要一起清。session 不存在回 404，`action`／`mode` 不合法回 400                                                                                                                                                                                                                                                                                                                                                                                          |
| `POST`                | `/v1/session/:id/followup`                  | **local** — 依對話產生後續建議 `suggests`，session 尚無標題時一併產生並寫入 `title`，回 `{title, suggests}`；無歷史時回空值，session 不存在回 404 |
| `GET`                 | `/v1/session/:id/task/history`              | **local** — 該 session 已完成的任務清單（新到舊），每列 `{task_hash, end_at, objective, model, reasoning}`；`?keyword=` 對 objective 與紀錄內容做過濾                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `GET`                 | `/v1/session/:id/task/:task_hash/history`   | **local** — 單一已完成任務的完整 action 紀錄，以 JSON 字串放在 `content`；該 hash 沒有紀錄時回 404                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |

**Channel**

| Method   | Path                                         | 說明                                                                                                                                                                                                                                                                                                                                        |
| -------- | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/v1/channel`                                | **local** — 所有 channel 讀取合在同一個物件：`telegram` 與 `discord` 各帶 `{enabled, username, has_token}`，`admin` 帶 `{channel, authorized, chats:[{value,type,id,name}]}`。`chats` 取自 `.telegram` / `.discord` 授權檔（先 tg 後 dc），`value` 可直接回送 `POST`；`authorized` 標示現行轉發目標是否仍在名單內（手打的 ID 會是 `false`） |
| `POST`   | `/v1/channel/telegram` `/v1/channel/discord` | **local** — `{action:"enable"\|"disable", token?}`。disable 會從 keychain 刪除 token 並清除使用者名稱；enable 只存 token 並切換設定 flag,刻意不做 TUI 那套 `GetMe` 驗證——daemon 既有的設定檔監看機制會自動重連 bot 並填回使用者名稱                                                                                                                                                            |
| `GET`    | `/v1/channel/:channel/chats`                 | **local** — `telegram` / `discord` 已完成驗證的對話，回 `{chats:[{value,type,id,name}]}`，來源是 `.telegram` / `.discord` 授權檔；未知頻道回 400。只有 bot 執行中才有意義，建議在 `GET /v1/channel` 回報 `enabled` 後才取用 |
| `DELETE` | `/v1/channel/:channel/chat`                  | **local** — `{id}`。從該授權檔移除一筆對話,該對話需重新驗證才能再與 bot 對話。id 不在名單上回 404                                                                                                                                                                                                                                           |
| `POST`   | `/v1/channel/admin`                          | **local** — `{value:"tg@<chatID>"\|"dc@<channelID>"\|""}`。設定新對話驗證碼的轉發目標,空字串清除;`value` 必填(省略回 400,避免誤送空 body 靜默清除)。只驗格式,不檢查該 ID 是否已在授權名單——未授權時 `NotifyAdminCode` 會 log warning 並讓驗證碼留在日誌                                                                                     |

**檔案與憑證**

| Method         | Path              | 說明                                                                                   |
| -------------- | ----------------- | -------------------------------------------------------------------------------------- |
| `GET` `PUT`    | `/v1/file`        | **local** — 讀取／寫入檔案                                                             |
| `GET`          | `/v1/file/open`   | **local** — 以系統預設程式開啟 `?path=` 指定、位於 home 內的檔案（敏感路徑回 403）；不接受 URL |
| `GET`          | `/v1/file/locate` | **local** — 依純檔名搜尋候選路徑（`name`,可選 `dir=1`、`child`、`size`、`mtime` 過濾） |
| `GET`          | `/v1/workdir`     | **local** — 解析並驗證工作目錄（`?path=`）,回傳絕對路徑                                |
| `DELETE`       | `/v1/key`         | **local** — 依 `?key=` 刪除 keychain 中單一憑證，並從已記錄的鍵名清單移除。沒有讀取端點：憑證只能寫入與列出名稱 |
| `GET` `POST`   | `/v1/keys`        | **local** — 列出已記錄的憑證名稱，或以 `{key, value}` 寫入 keychain                    |

**Provider**

| Method   | Path                             | 說明                                                                                                                                                                                                                                                                                                                                                                     |
| -------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET`    | `/v1/providers`                  | **local** — 列出 provider 及其可用操作                                                                                                                                                                                                                                                                                                                                   |
| `GET`    | `/v1/providers/quota`            | **local** — `claude-code`（解析 `claude -p /usage`，不打 API）、`codex`、`grok-oauth`、`copilot` 的剩餘額度與 `openrouter`、`deepseek` 的剩餘餘額，回 `{quota:{<id>:{remaining:{five_hour?,week?,total?,balance?}}}}`，平行取得，上限 10 秒。成功的結果在 ToriiDB 快取 3 分鐘並帶 `cached:true`；`?refresh=1` 清除快取重讀，存入 API key 或完成 OAuth 也會自動清掉該 provider 的快取。取不到的 provider 回 `error` 而非 `remaining`，且不進快取 |
| `GET`    | `/v1/model/quota?model=<name>`   | **local** — 單一模型所屬 provider 的最新剩餘額度，每次呼叫都重新查詢（逾時 15 秒）：`{"quota":"61%/40%"}`（5h／週／總額度百分比以 `/` 串接，餘額型為 `$12.34`），provider 不提供額度時回 `""`。Web 聊天在每次完成後呼叫 |
| `POST`   | `/v1/provider/:provider/key`     | **local** — 設定 API key。其他 provider 的 body 為 `{api_key}`，存為 `<PROVIDER>_API_KEY`；`cloudflare` 另需 `account_id`（可選 `gateway_id`）；`compat` 的 body 為 `{name, url, api_key?}`：網址記錄到 `compats`，有帶 key 時存為 `COMPAT_<NAME>_API_KEY`；不支援 API key 的 provider 回 400                                                                                                                                                                                                                                     |
| `GET`    | `/v1/provider/:provider/oauth`   | **local** — SSE OAuth 登入：`copilot` 為 device code（送 `{url, user_code}`），`codex`／`grok-oauth` 送瀏覽器 `{url}`，`claude-code` 只檢查 `claude` 執行檔；結束送 `{done, ok, error?}`，不支援 OAuth 的 provider 回 400 |
| `DELETE` | `/v1/provider/:provider/oauth`   | **local** — 清除已儲存的 provider 登入（`codex`、`copilot`、`grok-oauth`）。token 的 keychain 鍵名由 OAuth 套件自己持有（`CODEX_OAUTH_TOKEN` 與各自的舊名）,因此改走它們的 `ClearToken`,而非 `DELETE /v1/key`                                                                                                                                                            |
| `GET`    | `/v1/provider/:provider/models`  | **local** — 以該 provider 的憑證向其官方端點列出可用模型（`claude-code` 透過本機 `claude` CLI），回 `{models, windows}`；`windows` 為 model id 對 `{in, out}` context 大小的對照，取自 `llm-io.agenvoy.com`，查無資料的模型不會出現。本機／自訂端點名稱（`ollama`、`llama.cpp` 或已記錄的名稱）會向該端點的 `GET /models` 查詢。上游失敗回 502，不支援列模型的 provider 回 400；清單快取 15 分鐘 |
| `GET`    | `/v1/provider/:provider/console` | **local** — 以 302 導向 provider 的主控台：`?page=key` 為 API key 頁、`plan` 為訂閱方案頁；provider 有方案頁時預設 `plan`，否則 `key`；該頁不存在時回 404                                                                                                                                                                                                                |

**MCP**

| Method       | Path                     | 說明                                                                                                                                                                                                                            |
| ------------ | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET` `POST` | `/v1/mcp`                | **local** — 列出／新增 MCP server。`GET` 另回 `oauth: {name: bool}`,標示各 HTTP server 是否已持有 token                                                                                                                         |
| `POST`       | `/v1/mcp/remove`         | **local** — 移除 MCP server                                                                                                                                                                                                     |
| `GET`        | `/v1/mcp/status`         | **local** — 各 server 連線狀態                                                                                                                                                                                                  |
| `POST`       | `/v1/mcp/reconnect`      | **local** — 重連全部 MCP client 並重新註冊工具                                                                                                                                                                                  |
| `GET`        | `/v1/mcp/oauth?name=X`   | **local** — 單一 HTTP MCP server 的 SSE OAuth 登入,與 `/v1/provider/:provider/oauth` 同形狀:先送 `{"url":...}` 供瀏覽器開啟,結束送 `{"done":true,"ok":...}`(登入後重連失敗時附 `reconnect_error`)。10 分鐘逾時,客戶端斷線即中止 |
| `POST`       | `/v1/mcp/oauth/callback` | **local** — `{name, url}`。瀏覽器連不到 daemon 的 `localhost:17988` loopback listener 時,把 redirect URL 貼回來,code 由 query 取出。該 server 沒有等待中的登入回 400                                                            |
| `POST`       | `/v1/mcp/oauth/client`   | **local** — `{name, client_id, client_secret?, redirect_uri?}`。給拒絕動態註冊的 server 用的預先註冊 client;`redirect_uri` 預設 `http://localhost:17988/callback`,須與 provider console 完全一致。寫入前先清掉既有 token        |
| `DELETE`     | `/v1/mcp/oauth`          | **local** — `{name}`。同時清除該 server 的 token 與 client 註冊                                                                                                                                                                 |

**Rule 與 Skill**

| Method                  | Path              | 說明                                                            |
| ----------------------- | ----------------- | --------------------------------------------------------------- |
| `GET`                   | `/v1/roles`       | **local** — 列出 `prompts/` 底下的 session prompt role（`.md`）；payload 同時放在 `roles` 與相容用的 `rules` |
| `GET`                   | `/v1/role/*name`  | **local** — 讀取單一 role                                                                                     |
| `POST` `PATCH` `DELETE` | `/v1/role`        | **local** — 建立／更新（可帶 `rename`）／刪除 role                                                            |
| —                       | `/v1/rule[s]`     | **local** — 上述五條路由的舊名稱，仍指向同一組 handler |
| `GET`                   | `/v1/skills`      | **local** — 列出已安裝的 skill                                  |
| `GET`                   | `/v1/skill/*name` | **local** — 讀取單一已安裝的 skill                              |
| `DELETE`                | `/v1/skill`       | **local** — 移除單一已安裝的 skill                              |

**排程與自動化**

| Method         | Path                  | 說明                                                                                                                                                                                            |
| -------------- | --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`          | `/v1/schedule`        | **local** — 以單一 `schedules` 陣列列出 cron 與單次任務，每筆帶 `type=cron\|task`；`?type=` 可只取其一                                                                                          |
| `GET`          | `/v1/schedule/*skill` | **local** — 讀取 scheduler skill，回 `name`／`body`（SKILL.md 原文，含 frontmatter）／`files`（該資料夾內的其他檔案）                                                                           |
| `POST` `PATCH` | `/v1/schedule`        | **local** — 由 `name`／`content` 建立／更新 scheduler skill（`content` 自帶 frontmatter 則原樣寫入，否則由後端組出）並整組重綁到 `type=cron\|task`；切換 type 時同步刪除該 skill 在另一邊的排程 |
| `DELETE`       | `/v1/schedule`        | **local** — 刪除該 skill 的排程（帶 `type` 只刪一邊，不帶則兩邊都刪）；刪除後若無其他綁定則將 skill 移入 .Trash                                                                                 |
| `POST`         | `/v1/schedule/run`    | **local** — 立即觸發排程（`202 Accepted`）                                                                                                                                                      |

建立與更新的 body 為 `{type, name, content, expressions[], session_id?}`：`name` 須符合 `[A-Za-z0-9_-]{1,64}`；`type=cron` 時 `expressions` 為 cron 字串，`type=task` 時為未來的時間點。`POST` 遇到同名回 409，`PATCH` 找不到回 404；`DELETE` 收 `{skill, type?}`，沒有刪到任何排程回 404，否則回 `{removed, trashed}`；`run` 收 `{session_id, skill}`。

**白名單**

| Method       | Path            | 說明                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------ | --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET` `POST` | `/v1/allowlist` | **local** — 兩份白名單合在同一個物件，鍵為 `skill` 與 `tool`。`GET` 以 `?scope=global\|project`（`project` 需另帶 `?work_dir=`）決定 skill 區塊，`?prefix=` 縮小 tool 區塊。`POST` 收 `{skill: {name, scope?, work_dir?}}` 切換單一 skill，與／或 `{tool: {prefix, entries}}` 只替換該前綴底下的免確認項目（與 TUI `/mcp` → tools 同一支），其餘規則不受影響；每個 entry 必須以 `prefix` 開頭，出現 `prefix*` 時收斂成單一項。未帶的區塊不動 |

**設定**

| Method       | Path                    | 說明                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------ | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET` `POST` | `/v1/config/startup`    | **local** — 讀取／設定登入時啟動。`POST` `{enable}` 寫入或刪除 launchd agent（macOS）／systemd user unit（Linux）；不會啟動或停止當前 daemon，下次登入才生效。兩個動詞都回 `enabled`（設定值，每次變更時記錄於 `config.json` 的 `startup` 鍵）與 `installed`（unit 檔目前是否真的存在）——unit 被 Agenvoy 以外的方式移除時兩者會不一致                                                                                                                                                                                                                                          |
| `GET` `POST` | `/v1/config/system`     | **local** — 讀取／設定 System 分頁。`GET` 回 `{reply_lang, languages:[{code,label}]}`，`languages` 為 select 選項且 `auto` 排第一。`POST` `{reply_lang}` 會將已知代碼正規化、寫入 `config.json` 並立即套用到執行中的 daemon；空字串等同 `auto`，未知值原樣保留並當作語言名稱交給模型                                                                                                                                                                                                                                                                                           |
| `GET`        | `/v1/config/reply_lang` | **local** — 唯讀取得已設定的回應語言，形狀直接供 prompt 使用。回 `{reply_lang, name, directive}`：`reply_lang` 為儲存的代碼、`name` 為人類可讀語言名稱（`auto` 時為空）、`directive` 為現成的指示句並含地區用詞注記（`auto` 時為空，代表跟隨每則訊息的語言）。設定值請走 `POST /v1/config/system`                                                                                                                                                                                                                                                                              |
| `GET` `POST` | `/v1/config/output_dir` | **local** — 讀取／設定 `output_dir`。`GET` 回 `{output_dir, resolved}`，`resolved` 為實際使用中的資料夾。`POST` `{output_dir}` 會展開 `~`、建立資料夾、寫入 `config.json` 並立即套用到執行中的 daemon，回 `{ok, output_dir, resolved}`；空字串恢復預設值，建立不了的路徑回 400                                                                                                                                                                                                                                                                                                 |
| `GET` `POST` | `/v1/config/official_guide` | **local** — 讀取／設定是否注入官方模型指引。`GET` 回 `{enabled}`；`POST` `{enable}` 寫入 `config.json` 的 `official_guide_disabled` |
| `GET` `POST` | `/v1/config/skill_source` | **local** — 讀取／設定外部 skill 來源。`GET` 回 `{sources, disabled}`；`POST` `{disabled:[...]}`，未知來源回 400，儲存後重新掃描 skill |
| `GET` `POST` | `/v1/system/update`     | **local** — `GET` 回 `{version, latest, update_available}`，`latest` 為 GitHub latest release 轉址指向的 tag，兩者不同即 `update_available` 為 true；取不到 release 時回 502 `{version, error}`。`POST` 開啟終端機執行 `agen update`，回 202 `{status:"opened"}`：macOS 透過 `osascript` 開 Terminal.app，WSL 以 `cmd.exe start wsl.exe` 進入目前的 distro，Linux 使用第一個可用的終端機模擬器（需要 `DISPLAY` 或 `WAYLAND_DISPLAY`，daemon 環境缺少時從 systemd user environment 取得）。更新失敗時視窗保持開啟直到按 Enter。無法開啟終端機時回 501，請手動執行 `agen update` |

設定路由實際為單一的 `GET`／`POST /v1/config/:target`；未知的 target 回 404 `{"error":"unknown config target"}`，`reply_lang` 只能讀取。

**查閱**

| Method  | Path              | 說明                                                                                                                                                                                 |
| ------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET`   | `/v1/torii/error` | **local** — 查閱工具錯誤記憶（Web 的 **Lessons** 分頁）。未帶 `keyword` 時列出紀錄，可用 `tool` 縮小範圍（`limit` 預設 50）；帶 `keyword` 時走與 agent 相同的搜尋（`limit` 預設 16） |
| `PATCH` | `/v1/torii/error` | **local** — `{id, action}`。改寫單筆 lesson 的 action；id 已不存在時回 404                                                                                                           |
| `GET`   | `/v1/daemon`      | **local** — 近 28 天的 `daemon.log`，放在 `content`；可用 `from`／`to`（`yyyy-MM-dd-HH-mm`）與 `keyword` 過濾                                                                        |

## 工具參考

註冊表有 28 個工具固定可用（含內部用的 `run_tool`），另有 6 個依前置條件註冊或從執行中排除。涵蓋多種相關動作的工具以 `mode` 參數區分，而不是拆成多個名稱。

| 分類       | 工具                              | 用途                                                                                                                                                                                                                                                                                      |
| ---------- | --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 工具系統   | `find_tools`                      | 發現既有工具並以工具回傳交付其 schema（`mode=search\|list`） |
|            | `run_tool`                        | 依名稱與 args 執行 TurnContext 工具清單中的工具（呼叫前先以 `find_tools` 取 schema）；`claude-code` provider 不提供 |
|            | `edit_tool`                       | 建立、修正或丟棄工具定義（`mode=write\|patch\|remove`）                                                                                                                                                                                                                                   |
|            | `test_tool`                       | 上線前在沙箱內執行 script 工具                                                                                                                                                                                                                                                            |
| Skill      | `run_skill`                       | 載入具名 skill 的參考素材                                                                                                                                                                                                                                                                 |
|            | `edit_skill`                      | 編寫 skills 目錄底下的檔案（`mode=write\|patch\|remove`）                                                                                                                                                                                                                                 |
| 排程       | `schedules`                       | 查詢、改期或取消定時與週期任務（`mode=list\|patch\|remove\|write`）                                                                                                                                                                                                                       |
| 檔案       | `find_files`                      | 以目錄、檔名樣式或內容定位（`mode=list\|glob\|search`）；glob 依修改時間由新到舊排列；search 以 `offset`／`limit` 分頁（每頁 256），回傳符合的路徑、次數與前 5 個命中行號（`output=files`），或帶行號的符合行，可加 `context` 前後文（`output=content`）；`multiline` 讓 regex 可跨行比對 |
|            | `read_files`                      | 批次讀取文字、PDF、DOCX、PPTX、CSV／TSV、圖片與音訊／影片（回傳逐字稿）；預設 2048 行，文字檔被截斷時結尾會標示下一個 `offset`；`around` 讀取文字檔指定行號前後各 `context` 行（預設 20）；同一路徑可在一次呼叫中重複出現 |
|            | `edit_file`                       | 建立、修改、移置或還原檔案（`mode=write\|patch\|remove\|restore`）；覆寫或修改既有檔案前，必須在同一次執行中先以 `read_files` 讀過，且檔案之後未在磁碟上被改動                                                                                                                            |
|            | `file_history`                    | 工具改過的每個檔案的版本紀錄（`mode=list\|read`）                                                                                                                                                                                                                                         |
|            | `write_result`                    | 將長篇成果存為 `report-<時間>.md` 或 `.html`（`format=md\|html`），放在輸出資料夾（`output_dir`；預設 `~/Downloads`，不存在時為 `~/.config/agenvoy/download`）；模型只提供內容。`html` 寫入前會先檢查：須掛載 QuickUI（`new QUI(`）、inline script 須可解析、script 不得直接寫 markup（`innerHTML`、`createElement` 等）、`T.init` 目標 id 須存在、`{{ }}` 與 `:if` 不得含運算式；任一不符即不寫入並回傳問題清單（未掛 QuickUI 時另附 `html_render` 規則） |
| 執行環境   | `run_command`                     | 在工作目錄以沙箱約束執行二進位並等待結束；watcher（`--watch`、`chokidar`，或會啟動 watcher 的 package script，含經 `sh -c` 與 `package.json` 展開者）在啟動前即拒絕，改由 `background_command` 執行。argv 命中唯讀白名單（`configs/jsons/read_only_command.json`，532 條，可用 `read_only_command` 追加）時不需確認，並與其他唯讀呼叫並行執行；`sh -c`／`bash -c` 腳本須每個內層指令都命中，且不含變數賦值、`export`、`printf -v`、算術與參數展開；版本查詢條目（`--version`、`-v`）須整個 argv 相符，git 全域旗標只收明列項；其餘除非命中使用者的 allow 規則或本輪全部允許，否則跳確認。argv 任一參數落在敏感路徑（`sensitive_path.json`）即取消命中，所以 `cat` 雖在清單內，`cat ~/.aws/credentials` 仍會跳確認。條目以 `bin`、`bin sub`、`bin sub sub` 三種形式比對，第四段永不參與比對，更長的形式只能收斂成這三種。沙箱禁網，`git clone`／`fetch`／`push`、`npm install`、`pip install`、`go mod download`、`curl`、`wget` 都會解析不到主機，除非該次呼叫帶 `network: true`；帶了就跳確認，也不再比對唯讀白名單。沙箱只允許寫入 `$HOME`，需寫入其外的絕對路徑時於 `write_paths` 宣告；root 擁有的目標以 argv `['sudo', '<binary>', ...]` 執行（不接受 sudo 參數，也不可放在 `sh -c` 內），會以 `sudo -n` 在沙箱外執行；`sandbox: false` 以使用者身分在沙箱外執行（Homebrew 需要，且不可加 sudo）。三者都須經使用者以系統密碼核准。權限錯誤找不到可放行的路徑時，回傳內容會提示改用 `sandbox: false` 重跑。`rm` 改為移至垃圾桶（`sudo rm` 會直接刪除）；macOS 的 `open` 改走 `open_file`。單純抓 URL 走 `fetch_page`／`http_request`；裝系統套件用 `sudo` 加 system prompt Host OS 行列出的套件管理器（該行也列出作業系統版本），或以 `sandbox: false` 執行 `brew` |
|            | `open_file`                       | 以系統預設處理程式開啟檔案、URL 或系統設定（如 `x-apple.systempreferences:`）；本機檔案免確認，URL 一律跳確認 |
|            | `download_file`                   | 下載二進位資產至磁碟                                                                                                                                                                                                                                                                      |
| Agent 協調 | `subagents`                       | 將子任務委派到獨立 session（`mode=invoke\|list`）；invoke 必填 `model`（可為 `auto`，未註冊或 `pass` 層級的模型會被拒絕），`reasoning` 預設 `auto`，不合法的值直接報錯；子 session 固定排除 `subagents`、`edit_file`、`write_result`、`html_template`、`generate_*`、`open_file`、`schedules`、`store_secret` 等寫檔或有持久副作用的工具 |
|            | `write_todo`                      | 使用者即時看得到的任務清單                                                                                                                                                                                                                                                                |
|            | `ask_user`                        | 暫停提問。TUI 為 inline 提問：執行不中斷、該次工具呼叫不設逾時，回答後接著跑。其他頻道則中止執行並把狀態寫成 pending 快照，由回答續跑（快照中 args 截 4 KiB、tool result 截 32 KiB） |
| 網路       | `search_web`                      | DuckDuckGo 結果與 Google News 標題一次取得；有 decision model 時剔除與請求無關的結果                                                                                                                                                                                                                                                |
|            | `fetch_page`                      | 取得完整頁面內容（markdown／html／json）；agent 會先查看網站的 `/llms.txt`，有列出對應的 `.md` 頁面就優先抓取；有 decision model 時，markdown 會去除頁面雜訊，與請求無關的頁面不交給模型                                                                                                                                                                                                                                                  |
|            | `http_request`                    | 原始 HTTP 呼叫，含 multipart 上傳                                                                                                                                                                                                                                                         |
|            | `public_api_list`                 | 免金鑰公開 API 目錄：列出分類，或某分類下每個 API 的說明、HTTPS 標記與文件網址；找到合適的就以 `fetch_page` 讀文件、`http_request` 呼叫，不建工具 |
|            | `html_template`                   | HTML 頁面範例庫（`mode=list\|read`）                                                                                                                                                                                                                                                      |
| 狀態       | `chat_history`                    | 本 session 的執行紀錄與對話（`mode=list\|tool_list\|tool\|read\|search`）                                                                                                                                                                                                                 |
|            | `error_history`                   | 跨 session 保留的工具失敗紀錄（`mode=search\|read\|write`）                                                                                                                                                                                                                               |
|            | `reasoning_guide`                 | 依 `topics` 一次取得多個主題的完整推理規則                                                                                                                                                                                                                                                               |
| 基礎支援   | `calculate`                       | 算術、單位與匯率換算                                                                                                                                                                                                                                                                      |
|            | `store_secret`                    | 遮蔽輸入並存入 keychain                                                                                                                                                                                                                                                                   |
| 條件註冊   | `generate_image`                  | 文字生成圖片並存檔——image generator 為 off 時排除                                                                                                                                                                                                                                         |
|            | `generate_audio`                  | 文字轉語音並存檔——未選 TTS 模型時排除                                                                                                                                                                                                                                                     |
|            | `list_chatbot`、`send_to_chatbot` | 跨頻道推送——daemon 啟動時有啟用 Telegram 或 Discord 才註冊 |
|            | `background_command`              | 在背景啟動指令，結束時帶著輸出接續任務，用於 watcher 與長時間工作；輸出行命中錯誤關鍵字（`keywords`，內建預設清單）時，指令執行中也會接續任務。執行期間的輸出寫在 `sessions/<id>/background/<task>.log`，結束後刪除——非 TUI 請求時排除 |
|            | `background_subagents`            | 在背景啟動 subagent，完成時帶著存好的報告接續任務，用於很可能超過 5 分鐘的子任務；與 `background_command` 共用每個 session 的背景任務上限——非 TUI 請求時排除 |

工具依名稱排序，tool payload 只帶 `find_tools`、`run_tool`、`reasoning_guide` 三個完整 schema，其餘工具只以名稱列在每輪注入的 TurnContext；模型先以 `find_tools` 取得 schema（以工具回傳內容交付，不寫回 tool payload），再經 `run_tool(name, args)` 呼叫。未先取 schema 就呼叫時，執行器會自動回傳該 schema 並要求重新呼叫；直接以名稱呼叫這三個以外的工具會被拒絕。因此整個 session 的工具序列化結果逐位元組不變，prompt 快取得以保留。`claude-code` provider 不使用 `run_tool`：只收到工具名稱與完整的 `find_tools` schema，取得 schema 後直接以名稱呼叫。`edit_file` 的 patch 模式每個 target 只接受 `{old_string, new_string}`（另可帶 `replace_all`）；`new_string` 取代 `old_string`，插入則是在 `new_string` 開頭重複 `old_string`。所有 target 都對寫入前的磁碟原始內容比對，因此列出順序不影響結果；`old_string` 在未帶 `replace_all` 時比對到多處、兩個 target 覆蓋同一段、某個 target 的 `old_string` 出現在先前 target 的 `new_string` 內，或整批套用後會把非空檔案清空，整批都會拒絕且不寫入；`old_string` 與 `new_string` 相同的 target 會略過並在回傳中註明，內容沒有變化時不寫檔。

## 架構

請參閱完整的 [Architecture](./architecture.md) 與繁體中文 [架構](./architecture.zh.md)。

## License

本專案採雙授權：開源使用適用 [AGPL-3.0](../LICENSE)，無法滿足其原始碼公開義務者可洽詢商業授權，詳見 [COMMERCIAL.zh.md](./COMMERCIAL.zh.md)。

---

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
