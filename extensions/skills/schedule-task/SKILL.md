---
name: schedule-task
description: 當使用者要求在未來特定時間或週期性執行某件任務時使用。觸發條件：相對延遲（「X分鐘後」、「X小時後」、「等一下」、「待會」、「稍後」）、明確時間點（「X點」、「X時」、「下午」、「晚上」、「明天」、「後天」）、週期性（「每X分鐘」、「每天」、「每小時」、「定時」、「固定」）。訊息同時包含時間意圖與要做的事時必定觸發，禁止直接立即執行任務。
---

# 排程任務執行器

**收到此任務後，禁止呼叫任何執行型工具（fetch_google_rss、search_web、fetch_page、api_* 等）。必須走排程流程。**

## 步驟

### 1. 解析意圖

從訊息中提取：

- **時間**：什麼時候執行
- **任務**：移除時間描述後，實際要做的事

時間轉換規則：

| 使用者說 | `at` 參數 |
|---|---|
| X 分鐘後 | `+Xm` |
| X 小時後 | `+Xh` |
| X 點 / 下午 X 點 | `HH:MM`（24 小時制） |
| 明天 X 點 | `YYYY-MM-DD HH:MM` |
| 每 X 分鐘 | cron `*/X * * * *` |
| 每天 X 點 | cron `MM HH * * *` |

### 2. 撰寫腳本

根據任務撰寫腳本，所有輸出用 `echo` / `print` 到 stdout（系統自動整理後轉送到 Discord）：

**.sh 模板**：
```sh
#!/bin/sh
# 範例：查詢 Yahoo Finance 股價
curl -s "https://query1.finance.yahoo.com/v8/finance/chart/2603.TW" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
r = d['chart']['result'][0]
price = r['meta']['regularMarketPrice']
print(f'股價：{price}')
"
```

**.py 模板**：
```python
#!/usr/bin/env python3
import urllib.request, json

# 範例：Google News RSS
url = "https://news.google.com/rss/search?q=台電&hl=zh-TW&gl=TW&ceid=TW:zh-Hant"
with urllib.request.urlopen(url) as r:
    print(r.read().decode())
```

**規範**：
- `.sh` 以 `#!/bin/sh` 開頭
- `.py` 以 `#!/usr/bin/env python3` 開頭
- 只用系統內建工具：`curl`、`python3`（含標準函式庫）
- 禁止呼叫 Discord API 或 webhook
- **腳本的 stdout 會經過另一個 AI agent 包裝後才送到 Discord**。因此輸出必須包含明確的任務說明，讓 agent 知道這是「要轉達給使用者的訊息」，而非對話。
  - ❌ 錯誤：`echo "你很棒"` → agent 不知道這是要轉達的提醒，可能誤解語意
  - ✅ 正確：`echo "定時提醒：使用者要求提醒自己『你很棒』"` → agent 清楚這是排程產出的訊息

### 3. 儲存腳本

呼叫 `write_script`：
- `name`：描述性檔名（`.sh` 或 `.py`）
- `content`：步驟 2 的腳本

記下回傳的實際檔名（含 timestamp 後綴）。

### 4. 設定排程

**一次性任務** → `add_task`：
- `at`：步驟 1 轉換後的時間
- `script`：步驟 3 的實際檔名
- `channel_id`：當前 Discord 頻道 ID（從對話 context 取得）

**週期性任務** → `add_cron`：
- `cron_expr`：步驟 1 轉換後的 cron 表達式
- `script`：步驟 3 的實際檔名
- `channel_id`：當前 Discord 頻道 ID（從對話 context 取得）

### 5. 回覆使用者

簡短告知排程時間與任務內容，末尾附上 ID。

**回覆格式（必須完整照抄，禁止省略 ID）**：
```
已設定排程，{時間描述}會{任務描述}。
-# ID: `{從工具回傳擷取的 8 碼 ID}`
```

**範例**：
```
已設定排程，兩分鐘後會提醒你計時到了。
-# ID: `a1b2c3d4`
```
