---
name: search-suitable-public-api
description: Search the curated Agenvoy public API list for an API that fits the current user need or skill context, then chain into the `api-tool-add` skill to register it under `~/.config/agenvoy/tools/api/`. Triggers when the agent lacks a tool for a data lookup (weather, currency, geocoding, dictionary, etc.), when the user says "找個 API"／"有沒有 XXX 的 API"／"我需要查 XXX 但你沒工具"／"add a public API for this", or when an upstream skill needs an external data source that is not yet wired.
---

# Public API Discovery & Auto-Register

從 Agenvoy 維護的公開 API 清單中挑選符合當前需求的 API，抓取其文件，整理為結構化 endpoint 描述，最後透過 `api-tool-add` skill 寫入 `~/.config/agenvoy/tools/api/`。

---

## 何時觸發

| 情境 | 範例 |
|---|---|
| Agent 當下缺 tool | 使用者問匯率／天氣／IP 地理／字典／笑話／詩詞，agent 環顧 tool list 無對應 |
| 使用者明說 | 「幫我找個翻譯 API」、「有沒有可以查股價的開源 API」、「add a weather API tool」 |
| 上游 skill 依賴外部資料 | 某 skill 流程需要查 XXX 但本機 tools/api 無 — 先跑本 skill 補齊 |

**不觸發**：使用者只想知道某 API 存不存在但**不要新增**（直接 grep 清單回答即可，不必啟動完整 5 步流程）— 此情境用 `api_public_api_list` tool 一次性查詢回答。

---

## 前置依賴

| 依賴 | 來源 |
|---|---|
| `api_public_api_list` | runtime tool（已內建於 `extensions/apis/public-api-list.json`），無須額外註冊 |
| `fetch_page` | runtime tool |
| `api-tool-add` skill | 同 repo `extensions/skills/api-tool-add/SKILL.md` |
| `AskUserQuestion` (`ask_user`) | runtime tool |

---

## 工作流程（六步）

### Step 1：澄清需求

明確抓出**主題詞**（例：weather／exchange rate／dictionary／quotes）+ **必要欄位**（例：「要支援台幣」「要含 5 日預測」「免費／免註冊」）+ **語言／地區**（如「繁中字典」「台灣股市」）。

需求模糊時 — `AskUserQuestion` 一次性追問：

```
question: "為了挑對 API，請補充細節："
header: "Need spec"
options:
  - label: "免費 + 免認證優先"   description: "auth 欄位為空的 API 優先" (default)
  - label: "可接受需 API key"   description: "後續會走 api-tool-add 的 auth 流程設 env"
  - label: "只要 HTTPS"          description: "排除 https=No 的 API"
multiSelect: true
```

### Step 2：取得分類清單

呼叫 `api_public_api_list` 帶 `type=category` 取 47 個分類字串。

對需求主題詞做**語義對應**（不是字面 match）：

| 需求 | 對應分類（範例）|
|---|---|
| 翻譯／字典／同義詞 | `Dictionaries`、`Text Analysis` |
| 股價／匯率／加密幣 | `Finance`、`Currency Exchange`、`Cryptocurrency` |
| 天氣／空氣品質 | `Weather`、`Environment` |
| 地圖／座標／IP 定位 | `Geocoding`、`Tracking` |
| 笑話／詩詞／趣味 | `Entertainment`、`Animals`、`Games & Comics` |
| 開發者工具 | `Development`、`Programming`、`Open Source Projects`、`Test Data` |
| 政府／公開資料 | `Government`、`Open Data` |
| 食物／飲料／餐廳 | `Food & Drink` |
| 健康／醫療 | `Health` |
| 新聞／文章 | `News`、`Books` |

挑 **≤ 3 個**最相關分類進入下一步。挑太多會稀釋候選品質。

### Step 3：列出候選 API

對每個選中分類呼叫 `api_public_api_list?category=<name>` 取得該分類下 API 陣列。

候選欄位結構：

```json
{
  "category": "Weather",
  "api": "Open-Meteo",
  "description": "Free Weather Forecast API for non-commercial use",
  "auth": "",
  "https": "Yes",
  "cors": "Yes",
  "url": "https://open-meteo.com/"
}
```

**過濾規則**（依 Step 1 偏好）：

| 偏好 | 過濾 |
|---|---|
| 免認證優先 | 先取 `auth == ""`；不足再放寬 |
| 只要 HTTPS | 剔除 `https != "Yes"` |
| CORS 不重要 | runtime daemon 走 server-side 呼叫，不受 CORS 限制 — `cors` 欄位**不**作為過濾條件 |

**已存在檢查**：對每個候選比對 `~/.config/agenvoy/tools/api/` 與 `extensions/apis/` 內已有檔案（用 `list_files`／`run_command` 跑 `ls`）— 同名／同 host 已存在則標註「⚠️ 已存在」並讓使用者決定是否重複註冊。

依**描述相關度**（LLM 判斷）+ HTTPS／免認證偏好排序，取 **≤ 5 個**候選。

### Step 4：使用者挑選

`AskUserQuestion` 列出候選，每選項格式 `<api name> · <auth or "免認證"> · <one-line description>`：

```
question: "找到下列候選 API，要註冊哪一個？"
header: "Pick API"
options:
  - label: "Open-Meteo · 免認證 · 全球天氣預測（非商業免費）"
    description: "https://open-meteo.com/  HTTPS:Yes  CORS:Yes"
  - label: "OpenWeatherMap · apiKey · 商業級 + 歷史資料"
    description: "https://openweathermap.org/api  HTTPS:Yes"
  - label: "WeatherAPI · apiKey · 即時 + 14 天預測"
    description: "https://www.weatherapi.com/  HTTPS:Yes"
  - label: "都不合適，重新搜尋"
    description: "回 Step 2 換分類或調整 spec"
multiSelect: false
```

選「都不合適」→ 回 Step 1／2 重來；連兩輪不合適 → 明確告知「公開清單無理想匹配，是否改走手動描述走 api-tool-add？」

### Step 5：抓取 API 文件 → 整理 endpoint 描述

**關鍵**：`url` 欄位是 landing page，**不是** API base URL — 必須抓文件才能拿到實際 endpoint。

對選中 API 的 `url` 用 `fetch_page` 抓取，從頁面內容萃取：

| 欄位 | 來源線索 |
|---|---|
| Base URL | 文件示例 `https://api.<service>.com/...` |
| Endpoint paths | "GET /v1/..." 例子 |
| Required／optional params | 表格或欄位說明 |
| Auth scheme | landing page 明示「API key」「Bearer」「免認證」 |
| Response format | 範例 JSON |
| Rate limit | 文件提及的請求上限（記入 description） |

若 landing page 為 GitHub repo → fetch README；若有 `/docs`／`/api`／`/swagger` 子路徑 → 再 fetch 一次。

文件取得失敗或結構混亂（≥ 2 次 fetch 仍無法萃取 endpoint）→ `AskUserQuestion`：

```
question: "<API name> 文件不易解析，要怎麼做？"
header: "Doc parse"
options:
  - label: "貼上 endpoint 描述"   description: "由你補完 URL／method／參數，我整理"
  - label: "改選其他候選"         description: "回 Step 4 換一個"
  - label: "放棄"
```

整理結果為**自然語言 endpoint 描述**（給下一步 api-tool-add 用），格式：

```
API 名稱: open_meteo_forecast
描述: Free non-commercial weather forecast — current + hourly + daily up to 16 days, no API key required.
Method: GET
URL: https://api.open-meteo.com/v1/forecast
Auth: 無
Required params:
  - latitude (number, 例 25.0330): 緯度
  - longitude (number, 例 121.5654): 經度
Optional params:
  - current (string, 例 "temperature_2m,wind_speed_10m"): 逗號分隔即時欄位
  - hourly (string, 例 "temperature_2m,precipitation"): 逗號分隔逐時欄位
  - daily (string, 例 "temperature_2m_max,sunrise"): 逗號分隔逐日欄位
  - timezone (string, default "auto"): 時區
Response: JSON
Rate limit: 10,000 requests/day for non-commercial use
```

multi-endpoint API（如 OpenWeatherMap 含 `/current`／`/forecast`／`/onecall`）→ 列出全部，讓 api-tool-add 各寫一檔；但**預設只挑使用者當下需求最直接命中的 1 個**，避免一次塞滿 unrelated tools。

### Step 6：交棒給 api-tool-add

呼叫 `run_skill` 啟動 `api-tool-add` skill，並以 Step 5 整理好的描述作為輸入，按其 5 關卡流程處理：

| api-tool-add Gate | 本 skill 已提供 / 仍需處理 |
|---|---|
| Gate 1（取得來源）| **已跳過** — 來源為「手動描述」並已提供完整文字 |
| Gate 2（解析）| 直接以 Step 5 結構化描述生成 `APIDocumentData` |
| Gate 3（host 檢查）| 公開 API host 通常為公網域名 — 多半略過；若候選 API 文件示例為 localhost／私網（罕見）仍走完整檢查 |
| Gate 4（auth）| 候選 API `auth` 欄位非空 → 必走 — 詢問環境變數名稱、auth type（bearer／apikey／basic）|
| Gate 5（試打）| **必跑** — 用 Step 5 提供的範例值試打；失敗回到本 skill Step 4 換候選或 Step 5 修正描述 |

**勿**自行寫檔到 `~/.config/agenvoy/tools/api/` — 一律透過 api-tool-add 完成（保持 Gate 5 試打驗證為單一強制路徑）。

---

## 完成回報

```
🔍 搜尋分類: Weather, Environment
📋 候選: Open-Meteo / OpenWeatherMap / WeatherAPI
✅ 選擇: Open-Meteo（免認證）
📄 文件解析: https://open-meteo.com/ → /v1/forecast
🔧 已交棒 api-tool-add → 試打通過 → 寫入 ~/.config/agenvoy/tools/api/open_meteo_forecast.json

下一步: 重啟 agen daemon（`agen stop && agen`）即可使用 api_open_meteo_forecast tool。
```

---

## 反幻覺檢查

1. **不憑記憶推測 endpoint**：候選 API 的 base URL／path／參數**必須**來自 `fetch_page` 結果，禁止寫「常見的 OpenWeatherMap endpoint 應該是 `/data/2.5/weather`」之類記憶版本。
2. **`url` 欄位非 API URL**：清單回的 `url` 永遠視為文件入口，不直接寫入 `endpoint.url`。
3. **`auth: ""` 不等於完全免費**：部分 API 文件層才提到 rate limit／註冊建議；記入 description。
4. **候選 ≤ 5**：超過會稀釋使用者注意力；不足 5 個是常態，不必湊數。
5. **api-tool-add 不可跳過**：寫入路徑唯一走 api-tool-add Gate 5 試打 — 本 skill 不直接 `write_file` 到 `tools/api/`。

---

## 參考

- 清單 API tool：`extensions/apis/public-api-list.json`（`api_public_api_list`）
- 上游服務：`https://public-api-list.agenvoy.com/`
- 寫入 skill：`extensions/skills/api-tool-add/SKILL.md`
- 寫入目標：`~/.config/agenvoy/tools/api/`
