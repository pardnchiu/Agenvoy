# 步驟 6：更新主索引 `.doc/version-generate/CHANGELOG.md`

## 初始化（若不存在）

```bash
mkdir -p .doc/version-generate
INDEX=".doc/version-generate/CHANGELOG.md"

if [ ! -f "$INDEX" ]; then
    cat > "$INDEX" <<'EOF'
# Changelog

所有版本記錄，最新在上。

EOF
fi
```

## Prepend 新版本

將新版本的一行條目 **插入到標題之後、第一個版本條目之前**，保持最新在最上。

### 條目格式

```markdown
- [v1.3.0](./v1.3.0.md) — 2026-04-25 — 1 feat, 1 fix, 1 refactor, 1 perf, 1 security
```

### 摘要統計規則

- 僅列出**升版相關**標籤：`BREAKING` / `FEAT` / `FIX` / `UPDATE` / `REFACTOR` / `PERF` / `SECURITY` / `ADD` / `REMOVE`
- 不列 `STYLE` / `DOC` / `TEST` / `CHORE`
- `BREAKING` 存在時顯示為 `⚠️ N breaking` 並置於最前
- 順序：`breaking → feat → fix → update → security → refactor → perf → add → remove`
- 標籤名稱以小寫顯示

### 範例

| 變更組成 | 索引條目摘要 |
|---|---|
| 2 FEAT, 3 FIX | `2 feat, 3 fix` |
| 1 BREAKING, 1 FEAT | `⚠️ 1 breaking, 1 feat` |
| 1 SECURITY, 2 FIX | `2 fix, 1 security` |
| 僅 DOC | （不更新索引，type=none 時不產出） |

## 最終索引範例

```markdown
# Changelog

所有版本記錄，最新在上。

- [v2.0.0](./v2.0.0.md) — 2026-05-10 — ⚠️ 1 breaking, 2 feat
- [v1.3.0](./v1.3.0.md) — 2026-04-25 — 1 feat, 1 fix
- [v1.2.3](./v1.2.3.md) — 2026-04-10 — 2 fix
```
