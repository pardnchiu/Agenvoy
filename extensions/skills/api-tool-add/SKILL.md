---
name: api-tool-add
description: Convert user-supplied API source (Swagger/OpenAPI JSON, cURL, or natural-language endpoint description) into Agenvoy APIDocumentData format and write each endpoint as a separate JSON file under `~/.config/agenvoy/tools/api/`. Triggers on requests like "add an API tool", "新增 api tool", "把這個 swagger 轉成 api tool", "註冊一個 API 給 agent 用". Handles auth schema (bearer / apikey / basic) and warns on intranet/localhost hosts before write.
---

# API Tool Adder

把任意 API 來源轉為 Agenvoy `APIDocumentData` 格式並落地至 `~/.config/agenvoy/tools/api/`，每個 endpoint 一個獨立 JSON 檔。

---

## 目標

| 項目 | 說明 |
|---|---|
| **輸入** | Swagger 2.x / OpenAPI 3.x JSON（URL 或本機路徑）、cURL 指令、或純文字 endpoint 描述（method／URL／參數） |
| **輸出** | 每 endpoint 一檔，寫入 `~/.config/agenvoy/tools/api/<tool_name>.json` |
| **格式** | Agenvoy `APIDocumentData`（非 OpenAI function-calling schema、非 Claude tool schema） |

---

## 互動流程（五關卡）

### Gate 1：取得 API 來源

User message **未含**任何來源資訊（URL／檔案路徑／cURL／endpoint 描述）時 — 用 `AskUserQuestion` 詢問：

```
question: "API 來源是哪一種？"
header: "Source"
options:
  - label: "Swagger / OpenAPI JSON 檔案"  description: "本機檔案路徑（例：./swagger.json）"
  - label: "Swagger / OpenAPI URL"        description: "遠端 JSON URL（例：https://api.example.com/openapi.json）"
  - label: "cURL 指令"                    description: "貼上完整 cURL 指令，由 skill 推導 endpoint"
  - label: "手動描述"                     description: "method / URL / 參數逐項輸入"
multiSelect: false
```

選後再追問**實際內容**（用 `AskUserQuestion` 第二輪取路徑／URL／cURL 字串／endpoint 描述）。

User message **已含**來源（如「轉這個 swagger https://...」或「幫我加這個 endpoint: POST /users {name, email}」）— **跳過** Gate 1 直接解析。

### Gate 2：解析來源

| 來源 | 解析方式 |
|---|---|
| 本機 JSON 檔 | `read_file` 讀取 → JSON parse → 列出 paths/operations |
| 遠端 URL | `fetch_page` 或 `send_http_request` 取得 → JSON parse |
| cURL | 由 LLM 抽取 method／URL／headers／body／query |
| 手動描述 | 由 LLM 結構化為 endpoint object |

每個 endpoint 視為一個 tool，`name` 命名規則：`<resource>_<action>`（snake_case，例：`user_list`、`order_create`），來源若已給 `operationId` 優先沿用並轉 snake_case。`api_` prefix **不要**手動加 — runtime translator 會自動 prefix（見 `internal/toolAdapter/api/translate.go:74`）。

### Gate 3：Host 檢查（intranet／localhost）

對每個 endpoint 的 URL host 做檢查。**命中以下任一**即視為非公開 host：

| 條件 | 範例 |
|---|---|
| Loopback | `localhost`、`127.0.0.1`、`::1` |
| Private IPv4 | `10.*`、`172.16-31.*`、`192.168.*` |
| Link-local | `169.254.*`、`fe80:*` |
| `.local` mDNS | `myhost.local` |
| 無 scheme 或 host 缺漏 | `/users`（純 path）、`api/v1`（無 host） |

命中 → 用 `AskUserQuestion` 詢問：

```
question: "偵測到 URL 指向本機/區網（<host>），這是正確的目標嗎？"
header: "Host check"
options:
  - label: "是，保留此 URL"     description: "本地開發環境或內部服務"
  - label: "否，請輸入正確主機" description: "替換為對外可達的 host（含 scheme，例 https://api.prod.example.com）"
multiSelect: false
```

選「否」→ 再用 free-text `AskUserQuestion` 取新 host，套用至此 endpoint 與其他同 host endpoints（同一批 swagger 通常共享 base URL）。

### Gate 4：Auth 設定

從來源偵測 auth 需求：

| 來源類型 | 偵測點 |
|---|---|
| Swagger 2.x | `securityDefinitions` + `security` |
| OpenAPI 3.x | `components.securitySchemes` + `security` |
| cURL | `-H "Authorization: Bearer ..."` 或 `-H "X-API-Key: ..."` |
| 手動描述 | 由 LLM 主動詢問是否需要 |

**偵測到需要 auth** 或**無法判斷時** → 用 `AskUserQuestion`：

```
question: "API 是否需要身份驗證？需要的話走哪種類型？"
header: "Auth type"
options:
  - label: "不需要驗證"                          description: "公開 API，無 auth header"
  - label: "Bearer Token"                       description: "Authorization: Bearer <token>"
  - label: "API Key（header）"                  description: "自訂 header（預設 X-API-Key）"
  - label: "Basic Auth"                         description: "Authorization: Basic <base64(user:pass)>"
multiSelect: false
```

選擇非「不需要」→ 後續詢問：

1. **環境變數名稱**（free-text）：例 `EXAMPLE_API_KEY`、`OPENAI_API_KEY`。轉換為 SCREAMING_SNAKE_CASE。runtime 透過 `os.Getenv(<env>)` 取值，**不**儲存 key 本身。
2. **API Key 類型額外問**：自訂 header 名稱（預設 `X-API-Key`，可直接 Enter 採預設）。
3. 提醒使用者：「請於 `.env` 或 shell 環境中設定 `<env>=<your_token>`，daemon 啟動時讀取。」

**勿**詢問或儲存 token／key／secret 明文值 — schema 只記 `env` 變數名，明文必須由使用者自行設定環境變數。

### Gate 5：試打驗證（**必跑、未通過禁止寫入**）

對每個 endpoint 用 `send_http_request` 真實打一次，**確認可達且回應結構合理**才允許進入寫入階段。**絕對禁止**跳過此關卡直接寫檔 — 未驗證的 schema 等於把無效 tool 推給 agent，後續使用者觸發時才在 runtime 炸開。

#### 試打前準備

1. **必填參數**取樣值：
   - swagger／OpenAPI 含 `example`／`examples`／`default` → 直接使用
   - cURL 來源 → 沿用 cURL 內的實際值
   - 手動描述 → `AskUserQuestion` 詢問每個 required 參數的測試值（一次性問完）
   - 仍取不到 → 對該參數套 type-aware fallback：`string`→`"test"`、`integer`→`1`、`boolean`→`false`；fallback 命中時於試打結果註記「使用 fallback 值」
2. **Auth env 檢查**（僅有 `auth` 的 endpoint）：
   - 透過 `run_command` 跑 `printenv <ENV_NAME>` 或 `sh -c 'echo "${ENV_NAME}"'` 確認非空
   - 未設定 → `AskUserQuestion` 二選一：
     - 「我先設定環境變數，再繼續」→ 等使用者設好回覆 `已設定`，再次 `printenv` 確認
     - 「先用測試值試打一次（不寫入 env）」→ free-text 取 token，僅本次試打使用、**不**寫入 schema、**不**落入 history（透過 `store_secret` tool 處理；不可走 `ask_user` 取 plaintext）

#### 試打執行

對每個 endpoint，組裝實際 HTTP 請求：

| 項目 | 來源 |
|---|---|
| URL | `endpoint.url` 將 `{var}` 以樣本值代入；`GET`／`DELETE` query 動態參數附加為 query string |
| Method | `endpoint.method` |
| Headers | `endpoint.headers` 靜態 header 全帶；`auth.type`=`bearer` → `Authorization: Bearer <env value>`、`apikey` → `<auth.header or X-API-Key>: <env value>`、`basic` → `Authorization: Basic <base64>` |
| Body | `POST`／`PUT`／`PATCH` 將非 path 動態參數組為 JSON body（`content_type=form` 例外，組為 form-urlencoded） |
| Timeout | 取 `endpoint.timeout`，缺省 30s |

呼叫 `send_http_request`，記錄回應 `status_code`／`headers`／`body`（截 1KB 預覽）。

#### 結果判定

| Status | 判定 | 行為 |
|---|---|---|
| `2xx` | ✅ 通過 | 進入 Gate 寫入 |
| `3xx` | ⚠️ 重導 | 若 `Location` 指向同 host 不同 path → 詢問是否改用最終 URL；跨 host → 視同失敗 |
| `400`／`422` | ⚠️ 參數錯但 endpoint 可達 | 印出回應 body → `AskUserQuestion`：「endpoint 連線正常但回 4xx（參數驗證錯）— 仍要寫入嗎？」（option：寫入／修改參數重試／放棄） |
| `401`／`403` | ❌ Auth 錯 | 印出回應 → 詢問是否更換 env 值或 auth type 重試；放棄則該 endpoint 不寫入 |
| `404` | ❌ Path 錯或 host 錯 | 顯示完整請求 URL → 詢問是否修正 URL 重試；放棄則不寫入 |
| `5xx` | ⚠️ 伺服器側問題 | 印出回應 → `AskUserQuestion`：「endpoint 可達但伺服器 5xx — 仍要寫入嗎？」（一般選寫入，因為 schema 本身正確） |
| `timeout`／`connection refused`／`no such host` | ❌ 不可達 | **禁止寫入**；提示使用者檢查 Gate 3 host／網路／VPN，可選回 Gate 3 修改 host 後重試 |
| 非預期 body 結構 | ⚠️ 通過但記錄 | response 非 JSON 但 `response.format=json` → 提示使用者考慮改 `format` 或保留現況 |

#### 試打輸出範例

```
🔍 試打 user_get
   GET https://api.staging.example.com/users/42
   Authorization: Bearer ***（STAGING_API_KEY）
   ↳ 200 OK · application/json · 1.2KB
   ✅ 通過

🔍 試打 order_create
   POST https://api.staging.example.com/orders
   body: {"product_id":"test","quantity":1}
   ↳ 422 Unprocessable Entity
   { "error": "product_id must be UUID" }
   ⚠️ 參數驗證錯但 endpoint 可達 → 待使用者決策
```

#### 為何此關卡不可省

| 風險 | 後果 |
|---|---|
| URL typo（如 `/usrs` 而非 `/users`） | 寫入後 agent 呼叫時才 404，使用者誤判 agent 邏輯壞 |
| Path 變數命名不一致（swagger 用 `userId`，schema 寫成 `user_id`） | 模板替換失敗、URL 帶字面 `{userId}` 出去 |
| Auth header 名稱錯（`X-Api-Key` vs `Apikey`） | 401 但 agent 不會自我修正 |
| `content_type` 與伺服器不符 | 415 Unsupported Media Type |
| host 錯（複製公司內網 swagger 卻指向公網）| `connection refused`，agent timeout 浪費 token |

試打捕捉以上所有類型，**比 schema 視覺檢查可靠百倍**。

---

## 輸出格式（嚴格遵守）

### ✅ 正確格式（`APIDocumentData`）

```json
{
  "name": "get_user",
  "description": "Retrieve user profile by ID.",
  "endpoint": {
    "url": "https://api.example.com/users/{user_id}",
    "method": "GET",
    "content_type": "json",
    "headers": {
      "X-Custom-Header": "value"
    },
    "query": {
      "include": "profile"
    },
    "timeout": 30
  },
  "auth": {
    "type": "bearer",
    "env": "EXAMPLE_API_KEY"
  },
  "parameters": {
    "user_id": {
      "type": "string",
      "description": "User identifier, matches {user_id} in URL path.",
      "required": true
    },
    "include_deleted": {
      "type": "boolean",
      "description": "Include soft-deleted users when true.",
      "required": false,
      "default": false
    }
  },
  "response": {
    "format": "json"
  }
}
```

### ❌ 錯誤格式（禁止輸出）

```json
{
  "name": "get_user",
  "input_schema": {
    "type": "object",
    "properties": { "user_id": { "type": "string" } },
    "required": ["user_id"]
  }
}
```

以上為 OpenAI / Claude function-calling 格式 — agenvoy adapter 不識別。

### 欄位規則

| 欄位 | 必填 | 規則 |
|---|---|---|
| `name` | ✅ | snake_case，不加 `api_` 前綴（runtime 自動補） |
| `description` | ✅ | 一句動詞開頭，描述用途；英文優先 |
| `endpoint.url` | ✅ | 完整 URL，path 變數用 `{var_name}` |
| `endpoint.method` | ✅ | `GET`／`POST`／`PUT`／`PATCH`／`DELETE` |
| `endpoint.content_type` | ✅ | 預設 `json`；form data 用 `form` |
| `endpoint.headers` | optional | 靜態 header；動態 header 走 parameter |
| `endpoint.query` | optional | 靜態 query；動態 query 走 parameter |
| `endpoint.timeout` | optional | 秒，預設 30 |
| `auth.type` | conditional | `bearer`／`apikey`／`basic`；無 auth 則整個 `auth` 區塊省略 |
| `auth.header` | conditional | 僅 `apikey` 可用；省略時預設 `X-API-Key` |
| `auth.env` | ✅（若有 auth） | 環境變數名（SCREAMING_SNAKE_CASE） |
| `parameters.<name>.type` | ✅ | `string`／`integer`／`number`／`boolean`／`array`／`object` |
| `parameters.<name>.description` | ✅ | 用途 + 範例值 |
| `parameters.<name>.required` | ✅ | `true`／`false`（明確標示，勿省略） |
| `parameters.<name>.default` | optional | 非必填參數建議給；型別需匹配 `type` |
| `parameters.<name>.enum` | optional | 限制可選值 |
| `response.format` | ✅ | 一律填 `json`（目前 adapter 僅支援 JSON 回應） |

### Path 變數與 parameters 對應

URL 含 `{var}` → `parameters` 必須有同名 entry 且 `required: true`。例 `/users/{user_id}/posts/{post_id}` → 同時宣告 `user_id`／`post_id`。

### Query／Body 參數歸屬

| HTTP method | 動態 parameter 位置 |
|---|---|
| `GET`／`DELETE` | URL query string |
| `POST`／`PUT`／`PATCH` | JSON body（除非 path 變數） |

runtime 自動依 method 分派，**不**需在 schema 標明位置。

---

## 寫入規則

### 路徑

```
~/.config/agenvoy/tools/api/<name>.json
```

其中 `<name>` 為 schema 內的 `name` 欄位（snake_case，不加 `api_` 前綴）。一個 endpoint 一個檔案。

### 寫前檢查

| 條件 | 行為 |
|---|---|
| 目錄不存在 | `run_command` 跑 `mkdir -p ~/.config/agenvoy/tools/api` |
| 同名檔已存在 | 用 `read_file` 讀現有內容比對；不一致時 `AskUserQuestion` 詢問「覆蓋／改名／略過」 |
| 多 endpoint 批次 | 全部處理完並回報清單；單一失敗不阻斷其他 endpoint |

### 寫入方式

用 `write_file`（path 為絕對路徑），content 為 pretty-printed JSON（兩空格縮排、`\n` 結尾）。

---

## 完成回報

每個 endpoint 寫入後輸出：

```
✅ <tool_name> → ~/.config/agenvoy/tools/api/<tool_name>.json
   <METHOD> <URL>
   auth: <type|none>  params: <required>/<total>
```

最後總結：

```
Wrote N tool(s) to ~/.config/agenvoy/tools/api/
重啟 agen daemon（`agen stop && agen`）即可載入。
```

---

## 反幻覺檢查（產出前必驗）

1. **JSON 合法**：`json.Marshal` 等價的合法 JSON（無尾逗號、雙引號）。
2. **必填欄位齊全**：`name`／`description`／`endpoint.url`／`endpoint.method`／`endpoint.content_type`／`response.format` 全有。
3. **Path 變數對齊**：URL 內每個 `{var}` 都有同名 `parameters` entry。
4. **Auth 完整性**：有 `auth` → `type` ∈ `{bearer, apikey, basic}` 且 `env` 非空。
5. **無禁止格式**：沒有 `input_schema`／`properties` 包裝層／OpenAI function 結構。
6. **Host 已確認**：intranet／localhost host 已通過 Gate 3 確認或替換。
7. **試打通過**：Gate 5 對該 endpoint 取得 2xx（或使用者明確允許的 4xx／5xx）。未試打或 unreachable → **拒絕寫入**。

---

## 範例：完整一次互動

User: `幫我把這個 swagger 轉成 api tool: /Users/me/swagger.json`

1. **Gate 1 略過**（已給檔案路徑）
2. **Gate 2**：`read_file` 讀 swagger → 解析 3 個 endpoint：`GET /users/{id}`、`POST /users`、`GET /health`
3. **Gate 3**：所有 URL `http://192.168.1.50:8080/...` 為區網 → `AskUserQuestion` 確認 → 使用者選「否」並輸入 `https://api.staging.example.com` → 三 endpoint URL 同步替換
4. **Gate 4**：swagger 含 `bearerAuth` security → 詢問 env 名稱 → 使用者答 `STAGING_API_KEY`
5. **Gate 5**：`printenv STAGING_API_KEY` 確認已設 → 三 endpoint 試打：
   - `user_get` 用 swagger example `id=42` → `200 OK` ✅
   - `user_create` 用 body `{"name":"test"}` → `422` 缺 email → 使用者選「修改參數重試」追加 `email` → `201` ✅
   - `health_check` → `200 OK` ✅
6. **寫入**：`user_get.json`／`user_create.json`／`health_check.json` 三檔（`health_check` 不掛 auth — swagger 對 `/health` 標 `security: []`）
7. **回報**：列出三檔路徑 + 試打結果摘要 + 提醒設 `STAGING_API_KEY` env

---

## 參考

- Runtime 載入點：`internal/tools/executor.go` `apiToolbox.Load(filesystem.APIToolsDir)`
- 型別定義：`internal/toolAdapter/api/translate.go` `APIDocumentData`
- Auth 實作：`internal/toolAdapter/api/execute.go insetAuth()`
- 內建範例：`extensions/apis/*.json`（coingecko／ip-api／open-meteo 等 10 個）
- 官方範本：`internal/toolAdapter/api/example.json`
