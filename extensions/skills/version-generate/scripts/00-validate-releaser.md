# 步驟 0：發版者驗證

強制要求 `git config user.name` / `user.email`，缺失即中止。

```bash
RELEASER_NAME=$(git config user.name)
RELEASER_EMAIL=$(git config user.email)

if [ -z "$RELEASER_NAME" ] || [ -z "$RELEASER_EMAIL" ]; then
    echo "ERROR: git user.name or user.email not set" >&2
    echo "Run:" >&2
    echo "  git config --global user.name \"Your Name\"" >&2
    echo "  git config --global user.email \"you@example.com\"" >&2
    exit 1
fi

# 可選：取得 GitHub handle（未登入時降級，不中止）
RELEASER_GITHUB=""
if command -v gh &>/dev/null && gh auth status &>/dev/null; then
    RELEASER_GITHUB=$(gh api user --jq '.login' 2>/dev/null || echo "")
fi
```

## 輸出變數

| 變數 | 必要 | 說明 |
|---|---|---|
| `RELEASER_NAME` | ✅ | `git config user.name` |
| `RELEASER_EMAIL` | ✅ | `git config user.email` |
| `RELEASER_GITHUB` | ❌ | `gh api user` 的 login，無 gh 時為空字串 |
