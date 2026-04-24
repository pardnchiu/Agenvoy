---
name: version-generate
description: 從最新的 git tag 到 HEAD 生成結構化變更日誌並推薦新版本。當使用者請求生成變更日誌、發行說明或版本升級文件時使用。
---

# 版本產生器

從最新的 git tag 到 HEAD 生成 `.doc/vA.B.C.md` 變更日誌（含 frontmatter 與可追溯欄位），同步維護 `.doc/CHANGELOG.md` 主索引。

## 工作流程

| 階段 | 內容 | 實作細節 |
|---|---|---|
| 0 | 發版者驗證（git config 缺失即中止） | [scripts/00-validate-releaser.md](./scripts/00-validate-releaser.md) |
| 1 | 版本偵測（最新標籤 + remote URL） | [scripts/01-detect-version.md](./scripts/01-detect-version.md) |
| 2 | 收集變更（Conventional Commits 優先） | [scripts/02-collect-changes.md](./scripts/02-collect-changes.md) |
| 3 | 分類標籤與版本升級規則 | [scripts/03-classify-and-bump.md](./scripts/03-classify-and-bump.md) |
| 4 | 產出 `.doc/NEW_VERSION.md`（frontmatter + 追溯） | [scripts/04-output-template.md](./scripts/04-output-template.md) |
| 5 | 更新 `.doc/CHANGELOG.md` 主索引 | [scripts/05-update-index.md](./scripts/05-update-index.md) |
| ∞ | 分類規則與邊界案例 | [scripts/06-rules-and-edge-cases.md](./scripts/06-rules-and-edge-cases.md) |

## 核心契約

### 硬性前置條件
- `git config user.name` 與 `user.email` 必須設定（步驟 0）
- BREAKING 變更必須附 Migration 指引（步驟 4，缺失則中止）

### 輸出
- `.doc/<NEW_VERSION>.md` — 完整 changelog（含 frontmatter）
- `.doc/CHANGELOG.md` — 主索引（prepend 最新版本）

### 版本映射（SemVer）

| 偵測到的標籤 | 版本影響 |
|---|---|
| `BREAKING` | MAJOR（+1.0.0） |
| `FEAT` | MINOR（+0.1.0） |
| `FIX` / `UPDATE` / `SECURITY` / `REFACTOR` / `PERF` / `ADD` / `REMOVE` | PATCH（+0.0.1） |
| `STYLE` / `DOC` / `TEST` / `CHORE` | 不升版（`type: none`，不更新索引） |

### 執行順序
步驟 0 → 1 → 2 → 3 → 4 → 5，任一階段前置條件未達即中止，不產出殘缺文件。
