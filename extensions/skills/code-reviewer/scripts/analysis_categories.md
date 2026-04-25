# Analysis Categories

偵測類別與嚴重度對照表。由 `analyze_code.py` 及其子分析器產生 `issues` 清單時使用。

## 1. Code Quality（AST-based）

| Issue | Detection | Severity |
|-------|-----------|----------|
| 過長函式 | 函式 > 50 行 | Medium |
| 過深巢狀 | 巢狀深度 > 3 層 | Medium |
| 未使用 import | AST 名稱引用分析 | Low |
| 大量連續註解 | ≥ 10 行連續單行註解 | Low |
| Go: `interface{}` | AST 偵測空介面 | Low |
| Go: 丟棄回傳值 | `_ = f()` 模式 | Medium |
| Python: bare except | `except:` 無類型 | Medium |
| JS/TS: eslint 規則 | 呼叫專案 eslint | High / Medium |

## 2. Security（Pattern-based）

| Issue | Detection | Severity |
|-------|-----------|----------|
| 硬編碼密鑰（關鍵字） | `password=/secret=/api_key=` 等 | Critical |
| 可疑高熵字串 | Shannon entropy ≥ 4.0，長度 ≥ 32，排除 UUID / MD5 / SHA1 / SHA256 / MIME type | High |
| SQL Injection | 字串拼接 / f-string / % 格式化 SQL | High |
| Command Injection | 拼接系統指令 | High |

> Security 偵測採保守策略：高嚴重度標註「需人工確認」，不作為自動化修復依據。

---

## Severity Levels

| Level | Description | Action |
|-------|-------------|--------|
| Critical | 硬編碼密鑰等確切安全漏洞 | 立即修復 |
| High | 疑似注入或高影響問題 | 優先處理 |
| Medium | 影響可維護性 | 計劃修復 |
| Low | 風格或最佳實踐 | 有空處理 |
