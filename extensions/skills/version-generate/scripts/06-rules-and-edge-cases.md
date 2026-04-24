# 分類規則與邊界案例

## 分類規則

1. **版本優先**：`BREAKING` > `FEAT` > `PATCH_TAGS` > `NO_BUMP`
2. **最高優先**：單一 `BREAKING` → MAJOR，無論其他標籤
3. **空白區段**：省略沒有變更的標籤區段
4. **首次發行**：無現有標籤 → `v0.1.0`
5. **無功能變更**：僅 `STYLE` / `DOC` / `TEST` / `CHORE` → 保持目前版本，仍產出檔案但標註 `type: none`
6. **群組合併**：同類變更合併到單一區段
7. **範圍階層**：`BREAKING` > `SECURITY` > `FEAT` > `FIX` > 其他 > `DOC`
8. **描述格式**：動詞開頭（Add、Fix、Update、Remove、Refactor）
9. **聚焦意圖**：描述變更內容及原因，而非個別檔案
10. **追溯格式**：每條變更尾綴 `(#PR, @author) [short_hash]`，缺失欄位省略對應括號

## 邊界案例

| 情境 | 處理方式 |
|---|---|
| 發版者 git config 缺失 | **步驟 0 中止**，不產出任何檔案 |
| 無 `gh` CLI 或未登入 | 降級，frontmatter 省略 `github` 欄位 |
| 帶有功能的新檔案 | `FEAT`（非 `ADD`） |
| 新功能的測試 | `TEST`（與 `FEAT` 分開） |
| 重新命名 + 修改 | `REFACTOR` + `UPDATE`（兩條目） |
| 刪除棄用 + 新增替代 | `REMOVE` + `FEAT` |
| `.gitignore` 變更 | `CHORE` |
| `go.mod` / `go.sum` / `package-lock.json` | `CHORE`（相依性管理） |
| 自標籤後無提交 | 跳過生成，輸出「無變更」訊息 |
| `BREAKING` 無 Migration 內容 | **中止**並要求補充，不產出殘缺文件 |
| Co-author trailer 含 AI | 寫入 `co_authors`，不計入 `contributors` |
| 僅 `NO_BUMP` 標籤（STYLE／DOC／TEST／CHORE） | 不更新 `.doc/CHANGELOG.md` 索引 |
