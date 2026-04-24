# 步驟 3 & 4：分類標籤與版本升級規則

## 分類標籤

| 標籤 | 範圍 | 版本影響 |
|-----|-------|----------------|
| `FEAT` | 新功能 | **MINOR** (+0.1.0) |
| `FIX` | 錯誤修正 | PATCH (+0.0.1) |
| `UPDATE` | 修改現有行為 | PATCH |
| `ADD` | 新增檔案／資源 | PATCH |
| `REMOVE` | 刪除 | PATCH |
| `REFACTOR` | 程式碼重構（無行為變更） | PATCH |
| `PERF` | 效能改善 | PATCH |
| `SECURITY` | 安全性修補 | PATCH |
| `STYLE` | 格式化 | — |
| `DOC` | 文件 | — |
| `TEST` | 測試 | — |
| `CHORE` | 維護（CI／相依） | — |
| `BREAKING` | 破壞性變更 | **MAJOR** (+1.0.0) |

## 版本升級規則

```
LATEST_TAG = vX.Y.Z (預設: v0.0.0)

IF LATEST_TAG == v0.0.0:
    NEW_VERSION = v0.1.0

ELSE:
    MAJOR_TAGS = [BREAKING]
    MINOR_TAGS = [FEAT]
    PATCH_TAGS = [SECURITY, FIX, UPDATE, ADD, REMOVE, REFACTOR, PERF]
    NO_BUMP    = [STYLE, DOC, TEST, CHORE]

    IF any MAJOR_TAGS:   NEW_VERSION = v(X+1).0.0
    ELIF any MINOR_TAGS: NEW_VERSION = vX.(Y+1).0
    ELIF any PATCH_TAGS: NEW_VERSION = vX.Y.(Z+1)
    ELSE:                NEW_VERSION = LATEST_TAG
```

**RELEASE_TYPE** 同步決定為 `major` / `minor` / `patch` / `none`（寫入 frontmatter）。

## 範例

- `LATEST_TAG = v1.2.3` + 偵測到 `FEAT` → `NEW_VERSION = v1.3.0`（type: minor）
- `LATEST_TAG = v1.2.3` + 偵測到 `FIX` → `NEW_VERSION = v1.2.4`（type: patch）
- `LATEST_TAG = v1.2.3` + 偵測到 `BREAKING` → `NEW_VERSION = v2.0.0`（type: major）
- `LATEST_TAG = v0.0.0` → `NEW_VERSION = v0.1.0`
- `LATEST_TAG = v1.2.3` + 僅 `DOC` / `CHORE` → `NEW_VERSION = v1.2.3`（type: none）
