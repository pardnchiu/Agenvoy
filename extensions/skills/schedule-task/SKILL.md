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

### 2. 查詢可用工具

執行 `run_command` 取得工具清單：

```bash
python3 -c "
import urllib.request, json, os
base = 'http://localhost:' + os.environ.get('AGENVOY_PORT', '17989')
with urllib.request.urlopen(base + '/v1/tools') as r:
    for t in json.load(r)['tools']:
        print(t['name'], '|', t['description'][:80])
"
```

回傳的 `tools[]` 陣列包含每個工具的 `name`、`description`、`parameters`。依任務需求從中選擇最合適的工具，確認必填參數。

### 3. 撰寫腳本

**所有腳本統一使用 Python 3，透過本機 Agenvoy API 完成任務，禁止直接呼叫外部 API。**

流程：
1. `POST /v1/tool/{tool_name}` — 呼叫步驟 2 選定的工具取得原始資料
2. `POST /v1/send` — 將資料交給 AI 生成最終摘要，stdout 即為送往 Discord 的內容

**Python 模板**：

```python
#!/usr/bin/env python3
import json, urllib.request, os

BASE = f"http://localhost:{os.environ.get('AGENVOY_PORT', '17989')}"

def call_tool(name, args):
    payload = json.dumps(args).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/tool/{name}",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("result", "")

def send(prompt):
    payload = json.dumps({"content": prompt, "sse": False, "exclude_tools": ["add_task", "add_cron"]}).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/send",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("text", "")

# 1. 呼叫工具取得資料（tool_name 與 args 由步驟 2 決定）
result = call_tool("TOOL_NAME", {"PARAM": "VALUE"})

# 2. 交給 AI 格式化，直接 print 到 stdout
print(send(f"定時任務：根據以下資料整理成摘要報告：\n{result}"))
```

可串接多個工具：多次呼叫 `call_tool`，將結果合併後一次送入 `send`。

**規範**：
- 僅用 Python 3 標準函式庫（`json`、`urllib`、`os`）
- 禁止呼叫 Discord API 或 webhook
- `send()` 的 prompt 必須包含「定時任務：」前綴

### 4. 儲存腳本

呼叫 `write_script`：
- `name`：描述性檔名（`.py`）
- `content`：步驟 3 的腳本

記下回傳的實際檔名（含 timestamp 後綴）。

### 5. 設定排程

**一次性任務** → `add_task`：
- `at`：步驟 1 轉換後的時間
- `script`：步驟 4 的實際檔名
- `channel_id`：當前 Discord 頻道 ID（從對話 context 取得）

**週期性任務** → `add_cron`：
- `cron_expr`：步驟 1 轉換後的 cron 表達式
- `script`：步驟 4 的實際檔名
- `channel_id`：當前 Discord 頻道 ID（從對話 context 取得）

### 6. 回覆使用者

**回覆格式（必須完整照抄，禁止省略 ID）**：
```
已設定排程，{時間描述}會{任務描述}。
-# ID: `{從工具回傳擷取的 8 碼 ID}`
```
