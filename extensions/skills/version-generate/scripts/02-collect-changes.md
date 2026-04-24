# 步驟 2：收集變更（Conventional Commits 優先）

## 2.1 原始資料收集

```bash
# 完整 commit 資料：hash|subject|author_name|author_email|body
# 使用 \x1f (Unit Separator) 分隔欄位, \x1e (Record Separator) 分隔記錄
git log $LATEST_TAG..HEAD --format='%H%x1f%s%x1f%an%x1f%ae%x1f%b%x1e'

# 區間內去重作者清單
git log $LATEST_TAG..HEAD --format='%an <%ae>' | sort -u

# Co-author trailer（pair programming / AI 協作）
git log $LATEST_TAG..HEAD --format='%(trailers:key=Co-authored-by,valueonly)' \
    | grep -v '^$' | sort -u
```

## 2.2 Conventional Commits 解析規則

每個 commit subject 依下列正則優先比對：

```
^(feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert|security)(\([^)]+\))?(!)?: (.+)$
```

- **Group 1**（type）→ 映射到內部標籤
- **Group 2**（scope，選填）→ 寫入 `Scope` 區段
- **Group 3**（`!`）→ 標記 BREAKING
- **Group 4**（description）→ changelog 條目內文

### 型別映射

| Conventional | 內部標籤 |
|---|---|
| `feat` | FEAT |
| `fix` | FIX |
| `perf` | PERF |
| `refactor` | REFACTOR |
| `docs` | DOC |
| `test` | TEST |
| `style` | STYLE |
| `chore` / `build` / `ci` | CHORE |
| `security` | SECURITY |
| `revert` | REMOVE |
| 任一 type + `!` 後綴 | 追加 BREAKING |

## 2.3 BREAKING 偵測（雙重來源）

1. Subject 含 `!` 後綴（例：`feat!: ...`）
2. commit body 含 `BREAKING CHANGE:` trailer

任一命中即加入 BREAKING 區段，並**強制觸發 Migration 區段產出**（缺失則中止）。

## 2.4 PR 編號擷取

squash-merge 的 subject 通常尾綴 `(#123)`：

```
\(#([0-9]+)\)$
```

命中則記錄 PR 編號，輸出格式 `描述 (#123, @author) [a3f9c2d]`。

## 2.5 Fallback：無 Conventional 格式時

若 subject 未命中 2.2 正則，退回 **diff 語意分析**（LLM 判讀檔案內容變更性質），依分類規則歸類。
