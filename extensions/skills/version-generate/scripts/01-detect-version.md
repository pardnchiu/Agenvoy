# 步驟 1：版本偵測

取得最新語義化標籤，並組出 GitHub compare link 前綴。

```bash
git fetch --tags --quiet 2>/dev/null

LATEST_TAG=$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -1)

if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v0.0.0"
fi

# 將 SSH / HTTPS git URL 正規化為 https://github.com/{owner}/{repo}
REMOTE_URL=$(git config --get remote.origin.url \
    | sed -E 's#(git@github\.com:|https://github\.com/)([^/]+/[^/]+)(\.git)?$#https://github.com/\2#')
```

## 輸出變數

| 變數 | 說明 |
|---|---|
| `LATEST_TAG` | 最新 `vX.Y.Z` 標籤；無標籤則為 `v0.0.0` |
| `REMOTE_URL` | `https://github.com/{owner}/{repo}`；非 GitHub remote 時為原始 URL |
