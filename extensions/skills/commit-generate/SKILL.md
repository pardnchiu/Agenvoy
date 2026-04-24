---
name: commit-generate
description: Generate bilingual (English + Traditional Chinese) commit message from git changes.
---

# Commit Message Generator

從 git diff 產生雙語 commit message（英文 subject + 繁體中文 body）。

## Input

僅處理 staged 變更。若無 staged 檔案，直接報錯並停止，不 fallback 到工作區 diff。

## Steps

1. 使用 Bash 工具執行 `git diff --cached` 取得 staged diff
2. 若輸出為空，回應：「目前沒有 staged 變更，請先執行 `git add` 選擇要提交的檔案。」並停止
3. 判斷是否為跨主題變更（見 **Multi-Topic Detection**）
4. 依 **Tag Upgrade Signals** 由上而下掃描訊號，命中即強制升級 Tag，未命中才可依意圖選 lower tag
5. 套用下方規則產生 commit message
6. 若命中跨主題：先輸出拆分建議，再輸出單一概括 message；否則直接輸出 message
7. 輸出為純文字，不加額外說明

## Multi-Topic Detection

**偵測條件（任一命中即視為跨主題）：**

- diff 同時觸及 2 個以上 primary tag 類別（`feat` / `fix` / `refactor` / `breaking` / `security`）
- 變更檔案橫跨 2 個以上語意無關的模組（以一級目錄或套件邊界判斷）
- 單次 commit 包含 3 個以上彼此無關的主題

**命中時輸出格式：**

```
⚠️ 偵測到跨主題變更，建議拆分為多次 commit：
- {主題 A 簡述}：{檔案清單}
- {主題 B 簡述}：{檔案清單}

若仍要合併，以下為概括描述：
tag: English description
tag: 繁體中文描述
```

**理由：** 單次 commit 應呈現單一意圖。混合主題使 `git log` 難以追溯、`git revert` 無法精準還原、code review 無法聚焦。格式化、純註解補充、依賴升級等輔助性改動可併入主 commit，不視為跨主題。

## Tag Upgrade Signals

**強制升級規則：** 依下列順序由上而下掃描訊號，命中即強制採用對應 Tag；即使其他部分看起來像 feat / update / chore，也不得降級。

### Breaking Signals（命中 → `breaking`）

- 刪除任何 exported / public 符號（function、class、method、type、constant、enum value）
- 已存在函式 / 方法簽章變更：參數型別、順序、新增必填參數、回傳型別改變
- HTTP API 變更：endpoint 刪除、URL 路徑改、response schema 欄位移除或型別變、status code 變更
- 環境變數或設定檔：刪除既有鍵、新增必填鍵、既有鍵的型別 / 格式變更
- 資料庫 migration 非向後相容：`DROP COLUMN`、必填欄位加在有資料的表、型別縮窄
- CLI / 指令參數：旗標刪除、新增必填位置參數、既有旗標語意變更
- Import path / package / module 結構變更導致既有引用失效

### Security Signals（命中 → `security`，除非同時命中 Breaking）

- 修補注入類漏洞：SQL / XSS / Command / LDAP / Template injection
- 驗證 / 授權邏輯修補：missing auth check、privilege escalation、JWT 驗證缺失
- 敏感資料洩漏：log / response / error message 移除密鑰、token、PII
- 硬編碼密鑰 / token / 預設密碼移除
- CSRF / CORS / CSP / HSTS 相關修補
- 相依套件 CVE 修補（從 `chore: upgrade` 升級為 `security`）

### 未命中時

依 Tag 優先序 `BREAKING` > `FEAT` > `FIX` > `SECURITY` > `UPDATE` > `REFACTOR` > `PERF` > others 中擇一，匹配 diff 的核心意圖。

## Classification Tags

| Tag | 適用情境 |
|-----|----------|
| `feat` | 新功能、新 endpoint、新元件 |
| `fix` | Bug 修正、錯誤處理、nil check |
| `update` | 修改既有行為、參數調整 |
| `add` | 新增檔案/資源（非功能） |
| `remove` | 刪除程式碼或檔案 |
| `refactor` | 重構（行為不變） |
| `perf` | 效能優化 |
| `style` | 格式化、排版 |
| `doc` | 文件、註解 |
| `test` | 測試相關 |
| `chore` | CI/CD、工具、依賴管理 |
| `security` | 安全性修補 |
| `breaking` | 破壞性變更 |

## Output Format
```
tag: English one-line description
tag: 繁體中文一句話描述
```

## Rules

1. **單一 Tag** — 選擇最能代表本次變更核心意圖的 Tag
2. **Tag 優先序** — `BREAKING` > `FEAT` > `FIX` > `SECURITY` > `UPDATE` > `REFACTOR` > `PERF` > others
3. **英文 subject** — imperative mood，不超過 72 字元（Add / Fix / Refactor / Remove / Update）
4. **繁體中文 body** — 不超過 50 字，動詞開頭（新增、修正、重構、移除、優化）
5. **合併相關變更** — 多個小改動歸納為單一描述
6. **單一主題優先** — 一次 commit 表達一個意圖；偵測到跨主題時先警示使用者拆分，再給概括 message

## Examples
```
feat: Add Docker environment auto-detection and database path switching
feat: 新增 Docker 環境自動偵測與資料庫路徑切換機制

fix: Fix token expiration not handled correctly during user login
fix: 修正使用者登入時 token 過期未正確處理的問題

refactor: Extract shared validation logic from order module
refactor: 重構訂單模組並抽離共用驗證邏輯

update: Set API timeout to 30s and pass context through call chain
update: 調整 API timeout 為 30 秒並加入 context 傳遞

chore: Upgrade Go to 1.22 and update dependencies
chore: 升級 Go 版本至 1.22 並更新相依套件

breaking: Remove legacy API v1 endpoints, all clients must update
breaking: 移除舊版 API v1 endpoint，需更新所有 client

security: Fix SQL injection via parameterized queries
security: 修補 SQL injection 漏洞並加入參數化查詢

perf: Optimize batch queries to eliminate N+1 problem
perf: 優化批次查詢減少 N+1 問題
```
