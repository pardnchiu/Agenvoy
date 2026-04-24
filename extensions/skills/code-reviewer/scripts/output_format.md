# Output Format

報告結構範本與撰寫規則。產生 `{yyyy-MM-dd_HH-mm}.md` 時套用此範本。

## Report Structure

```markdown
# {project_name} 優化建議報告

## 摘要

- 語言：{language}
- 檔案數：{file_count}
- 函式數：{function_count}
- 問題總數：{issue_count}（Critical: X, High: X, Medium: X, Low: X）

---

## Critical Issues

### 1. {issue_title}

**檔案**：`{file_path}:{line_number}`

**問題**：{description}

**目前程式碼**：
```{lang}
{current_code}
```

**建議修改**：
```{lang}
{suggested_code}
```

**原因**：{reason}

---

## High Priority Issues

...

## Medium Priority Issues

...

## Low Priority Issues

...

---

## 架構建議

...

## 效能優化建議

...

## 安全性強化建議

...

## 待處理項目清單

- [ ] {task_1}
- [ ] {task_2}
```

---

## Output Guidelines

1. **繁體中文** — 報告使用繁體中文（ZH-TW）
2. **技術術語** — 保留英文（Race Condition / Memory Leak 等）
3. **具體建議** — 提供可執行的修改建議，非泛泛之談
4. **優先排序** — 按嚴重程度排序問題
5. **程式碼範例** — 提供修正前後對比
