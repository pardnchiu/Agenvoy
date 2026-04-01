---
name: swagger-to-api
description: 解析 Swagger 2.x / OpenAPI 3.x JSON 檔，將每個 endpoint 轉換為 agenvoy APIDocumentData 格式並寫入 ~/.config/agenvoy/apis/。
---

# Swagger → Agenvoy API 轉換器

## ❌ 錯誤格式（禁止輸出）

以下任何格式都是**錯誤的**，不能使用：

```json
{
  "name": "get_user",
  "input_schema": {
    "type": "object",
    "properties": {
      "user_id": { "type": "string", "description": "..." }
    },
    "required": ["user_id"]
  }
}
```

以上是 Claude / OpenAI Function Calling 格式，**此 skill 絕對禁止輸出此格式**。

---

## ✅ 正確格式（agenvoy APIDocumentData）

每個 endpoint 必須嚴格按以下結構輸出，**每個 endpoint 各自寫入一個獨立 JSON 檔案**：

```json
{
  "name": "get_user",
  "description": "取得使用者資訊",
  "endpoint": {
    "url": "https://api.example.com/users/{user_id}",
    "method": "GET",
    "content_type": "json",
    "timeout": 30
  },
  "parameters": {
    "user_id": {
      "type": "string",
      "description": "使用者 ID",
      "required": true
    },
    "include_deleted": {
      "type": "boolean",
      "description": "是否包含已刪除的使用者",
      "required": false,
      "default": false
    }
  },
  "response": {
    "format": "json"
  }
}
```

**關鍵差異：**
- `parameters` 是 **flat object**，每個 key 是參數名，值含 `type`/`description`/`required`
- **沒有** `input_schema`、`properties`、`required` 陣列
- **沒有** `function`、`tool`、`type: "object"` 包裝層

### 含 auth 的範例

```json
{
  "name": "create_order",
  "description": "建立新訂單",
  "endpoint": {
    "url": "https://api.example.com/orders",
    "method": "POST",
    "content_type": "json",
    "timeout": 30
  },
  "auth": {
    "type": "bearer",
    "env": "EXAMPLE_API_KEY"
  },
  "parameters": {
    "product_id": {
      "type": "string",
      "description": "商品 ID",
      "required": true
    },
    "quantity": {
      "type": "integer",
      "description": "購買數量",
      "required": true
    },
    "currency": {
      "type": "string",
      "description": "貨幣代碼",
      "required": false,
      "default": "USD",
      "enum": ["USD", "TWD", "EUR"]
    }
  },
  "response": {
    "format": "json"
  }
}
```

---

## Permission

此 skill 已授權執行多次 `write_file` 呼叫，每個 endpoint 各呼叫一次，無需額外確認。

---

## Input

`args` 為 swagger/openapi JSON 檔案的路徑。若使用者附加檔案（Discord 附件），內容已嵌入對話中。

---

## Steps

### 1. 取得規格內容

**優先**：檢查對話中是否已有嵌入的檔案內容（以 `----------` 分隔線包裹的文字區塊）。
- 若有 → 直接使用該內容，**跳過 read_file**
- 若無 → 使用 `read_file` 工具讀取 args 指定的路徑；若失敗，回報錯誤並停止

### 2. 取得輸出路徑

執行 `echo $HOME` 取得 HOME 路徑，再執行 `mkdir -p $HOME/.config/agenvoy/apis`。

### 3. 判斷規格版本

- 含 `"swagger"` key → Swagger 2.x
- 含 `"openapi"` key → OpenAPI 3.x

### 4. 提取全域資訊

**OpenAPI 3.x**
- Base URL：`servers[0].url`（展開 variables 預設值）
- 若 `servers[0].url` 為相對路徑（以 `/` 開頭，不含 scheme）→ 停止執行，詢問使用者：「規格中未包含主機網址，請提供 base URL（例如 https://api.example.com）」，待使用者回覆後繼續
- Security Schemes：`components.securitySchemes`

**Swagger 2.x**
- Base URL：`(schemes[0] ?? "https") + "://" + host + (basePath ?? "")`
- 若 `host` 欄位缺失或為空 → 停止執行，詢問使用者：「規格中未包含主機網址，請提供 base URL（例如 https://api.example.com）」，待使用者回覆後繼續
- Security Schemes：`securityDefinitions`

### 5. 逐一轉換每個 endpoint 並立即寫檔

對 `paths` 中每個 `{path}` × `{method}` 組合：

#### 5.1 name

優先 `operationId` → snake_case；無則 `{first_tag}_{method}_{path_slug}`。只含 `[a-z0-9_]`，不得以數字開頭。

#### 5.2 description

`operation.summary` → `operation.description`（截 120 字元）→ `"{METHOD} {path}"`。

#### 5.3 endpoint.url

`base_url + path`，保留 `{param}` 佔位符。

#### 5.4 endpoint.method

HTTP method 全大寫。

#### 5.5 content_type

requestBody 含 form/multipart → `"form"`；其他 → `"json"`。

#### 5.6 timeout

預設 `30`。operationId 含 upload/export/report/generate → `120`。

#### 5.7 parameters（agenvoy flat object，非 JSON Schema）

合併 path params（`required: true`）、query params、requestBody properties。
跳過 `in: header` 和 `in: cookie`。

Type 對應：

| OpenAPI type | agenvoy type |
|---|---|
| integer, int32, int64 | integer |
| number, float, double | number |
| boolean | boolean |
| array | string（description 加 "comma-separated"） |
| object | string（description 加 "JSON string"） |
| string | string |

#### 5.8 auth

查 security 陣列取第一個 scheme：
- http bearer / oauth2 → `{ "type": "bearer", "env": "SCHEME_NAME_API_KEY" }`
- apiKey in header → `{ "type": "apikey", "header": "{name}", "env": "SCHEME_NAME_API_KEY" }`
- apiKey in query → `{ "type": "apikey", "header": "X-API-Key", "env": "SCHEME_NAME_API_KEY" }`
- 無 security → **省略 auth 欄位**

#### 5.9 response.format

produces/responses content 含 text/plain 或 text/html → `"text"`；其他 → `"json"`。

#### 5.10 呼叫 write_file（每個 endpoint 各一次）

組合 JSON 物件後，**立即**呼叫 `write_file`：
- 路徑：`{HOME}/.config/agenvoy/api_tools/{name}.json`（絕對路徑）
- 內容：符合「✅ 正確格式」的 JSON，**省略**值為空的選填欄位
- **每個 endpoint 獨立呼叫一次，不合併為陣列**

### 6. 輸出摘要

```
已轉換 {N} 個 endpoint，寫入 {HOME}/.config/agenvoy/apis/：
- {name}.json → {METHOD} {path}
```

---

## 注意事項

- 同名衝突（同 name 不同 endpoint）→ 附加 `_{method}` 區分
- `parameters` 為空時保留欄位：`"parameters": {}`
