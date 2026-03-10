> [!NOTE]
> 此 README 由 [SKILL](https://github.com/pardnchiu/skill-readme-generate) 生成，英文版請參閱 [這裡](./README.md)。

# swagger-to-api

> 解析 Swagger 2.x / OpenAPI 3.x JSON 規格，將每個 endpoint 轉換為 agenvoy APIDocumentData 格式並寫入本地設定目錄<br>
> 此 SKILL 主要由 [Claude Code](https://claude.ai/claude-code) 生成，作者僅做部分調整。

## 目錄

- [功能特點](#功能特點)
- [安裝](#安裝)
- [使用方法](#使用方法)
- [命令列參考](#命令列參考)
- [授權](#授權)

## 功能特點

### Swagger 2.x 與 OpenAPI 3.x 雙版本自動識別

自動偵測輸入規格的版本（Swagger 2.x 或 OpenAPI 3.x），並依版本差異正確提取 Base URL、Security Schemes 與所有 path/method 組合。無需手動指定版本，讓同一指令可無縫處理不同來源的 API 規格。

### 符合 agenvoy 規範的逐 endpoint 輸出

將每個 API endpoint 轉換為 agenvoy 專屬的 flat APIDocumentData 格式，而非 JSON Schema 或 Function Calling 格式，每個 endpoint 各自寫入獨立 JSON 檔案至 `~/.config/agenvoy/apis/`。這解決了不同工具格式不相容的問題，讓 agenvoy 能直接載入而無需二次轉換。

### Auth 與型別自動推導

自動將 OpenAPI Security Scheme（bearer、apiKey header/query、OAuth2）對應為 agenvoy auth 欄位，並將 OpenAPI 參數型別（array、object 等）轉換為 agenvoy 相容型別，附上語意化 description 補充說明。開發者無需手寫格式轉換邏輯，即可獲得立即可用的 API 定義檔。

## 安裝

將此技能放置於 Claude Code 的技能目錄：

```bash
~/.claude/skills/swagger-to-api/
```

目錄結構：

```
swagger-to-api/
├── SKILL.md              # 技能定義檔
├── LICENSE
├── README.md
└── README.zh.md
```

## 使用方法

```bash
/swagger-to-api <swagger_json_path>
```

### 使用範例

```bash
# 從本地檔案轉換
/swagger-to-api /path/to/openapi.json

# 使用者附加 Discord 附件時，內容已嵌入對話，直接執行即可
/swagger-to-api
```

## 命令列參考

### 參數

| 參數 | 必填 | 說明 |
|------|------|------|
| `swagger_json_path` | 否 | Swagger / OpenAPI JSON 檔案的路徑；若檔案內容已嵌入對話則可省略 |

### 支援的規格版本

| 版本 | 識別欄位 | Base URL 來源 |
|------|----------|---------------|
| Swagger 2.x | `swagger` key | `schemes[0]://host + basePath` |
| OpenAPI 3.x | `openapi` key | `servers[0].url` |

### 輸出檔案

| 檔案 | 說明 |
|------|------|
| `~/.config/agenvoy/apis/{name}.json` | 每個 endpoint 各一個 JSON 檔，格式為 agenvoy APIDocumentData |

### Auth 型別對應

| OpenAPI Security Scheme | agenvoy auth.type |
|------------------------|-------------------|
| http bearer / oauth2 | `bearer` |
| apiKey in header | `apikey`（含 header 名稱） |
| apiKey in query | `apikey` |
| 無 security | 省略 auth 欄位 |

## 授權

本專案採用 [MIT LICENSE](LICENSE)。
