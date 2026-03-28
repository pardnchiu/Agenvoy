---
name: script-tool-creator
description: Create a new Agenvoy script tool (JavaScript or Python) stored at ~/.config/agenvoy/script_tools/. Use when asked to create, add, or generate a script tool. A script tool is a local executable invoked by the agent via stdin/stdout JSON; it extends agent capabilities without modifying the core codebase. Triggers on phrases like "create a script tool", "add a script tool", "make a tool that runs locally", "generate a script tool".
---

# Script Tool 建立器

> **路徑規則**：所有 script tool 一律儲存至 `~/.config/agenvoy/script_tools/<tool-name>/`。`init_script_tool.py` 的 `--path` 固定使用 `~/.config/agenvoy/script_tools`。

> **輸出路徑預設值**：Script tool 若涉及下載、輸出或寫入檔案，當使用者未指定路徑時，依序使用：`~/Downloads`（存在則優先）→ `~/.config/agenvoy/download`（fallback）。實作範例：
> ```python
> downloads = os.path.expanduser("~/Downloads")
> default_path = downloads if os.path.isdir(downloads) else os.path.expanduser("~/.config/agenvoy/download")
> output_path = params.get("path", default_path)
> ```

## 關於 Script Tool

Script tool 是放在本地的可執行腳本，agent 在工具呼叫時透過 subprocess 執行：

- **參數**：以 JSON 字串從 **stdin** 傳入（即 `tool.json` 中定義的 parameters）
- **回傳**：結果輸出至 **stdout**（建議 JSON 格式）
- **命名**：工具呼叫名稱為 `script_<name>`（例如 `script_fetch_weather`）
- **載入**：agent 啟動時掃描，**新增後需重啟**

### 目錄結構

```
~/.config/agenvoy/script_tools/
└── <tool-name>/
    ├── tool.json      （必要）工具描述與參數 Schema
    └── script.js      （必要）JavaScript，或
        script.py               Python
```

### tool.json 格式

```json
{
  "name": "fetch_weather",
  "description": "取得指定城市的即時天氣資訊",
  "parameters": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "城市名稱"
      }
    },
    "required": ["city"]
  }
}
```

### 腳本 stdin/stdout 規範

```js
// JavaScript (script.js) — node 執行
const chunks = [];
process.stdin.on("data", (d) => chunks.push(d));
process.stdin.on("end", () => {
  const input = JSON.parse(Buffer.concat(chunks).toString() || "{}");
  // input = { city: "Taipei" }
  console.log(JSON.stringify({ result: "..." }));
});
```

```python
# Python (script.py) — python3 執行
import json, sys
params = json.loads(sys.stdin.read() or "{}")
# params = { "city": "Taipei" }
print(json.dumps({"result": "..."}))
```

## 呼叫現有工具

Script tool 內部可直接透過 Agenvoy API 呼叫現有工具，避免重複實作已有功能。

### 查詢可用工具

建立前先確認是否有現成工具可直接組合：

```bash
python3 -c "
import urllib.request, json, os
base = 'http://localhost:' + os.environ.get('AGENVOY_PORT', '17989')
with urllib.request.urlopen(base + '/v1/tools') as r:
    for t in json.load(r)['tools']:
        print(t['name'], '|', t['description'][:80])
"
```

回傳 `tools[]` 包含每個工具的 `name`、`description`、`parameters`。

### 腳本內呼叫工具（Python）

```python
import json, sys, urllib.request, os

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
    payload = json.dumps({"content": prompt, "sse": False}).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/send",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("text", "")

params = json.loads(sys.stdin.read() or "{}")

# 呼叫現有工具取資料
raw = call_tool("search_web", {"query": params["query"]})

# 需要 AI 格式化時走 /v1/send，否則直接 print
print(json.dumps({"result": send(f"整理以下資料：\n{raw}")}))
```

### 腳本內呼叫工具（JavaScript）

```js
const https = require("http");

const BASE = `http://localhost:${process.env.AGENVOY_PORT || 17989}`;

function callTool(name, args) {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify(args);
    const req = https.request(`${BASE}/v1/tool/${name}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) },
    }, (res) => {
      let data = "";
      res.on("data", (d) => (data += d));
      res.on("end", () => resolve(JSON.parse(data).result ?? ""));
    });
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

function send(prompt) {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify({ content: prompt, sse: false });
    const req = https.request(`${BASE}/v1/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) },
    }, (res) => {
      let data = "";
      res.on("data", (d) => (data += d));
      res.on("end", () => resolve(JSON.parse(data).text ?? ""));
    });
    req.on("error", reject);
    req.write(body);
    req.end();
  });
}

const chunks = [];
process.stdin.on("data", (d) => chunks.push(d));
process.stdin.on("end", async () => {
  const params = JSON.parse(Buffer.concat(chunks).toString() || "{}");
  const raw = await callTool("search_web", { query: params.query });
  const text = await send(`整理以下資料：\n${raw}`);
  console.log(JSON.stringify({ result: text }));
});
```

**使用原則**：
- `call_tool` 對應 `POST /v1/tool/{name}`，只需傳必填參數
- `send` 對應 `POST /v1/send`，需要 AI 格式化輸出才呼叫，純資料處理不需要
- 呼叫前先用 `GET /v1/tools` 確認工具存在與參數格式

---

## 建立流程

### 步驟一：查詢現有工具

執行 `run_command` 確認是否已有可直接組合的工具，避免重複實作：

```bash
python3 -c "
import urllib.request, json, os
base = 'http://localhost:' + os.environ.get('AGENVOY_PORT', '17989')
with urllib.request.urlopen(base + '/v1/tools') as r:
    for t in json.load(r)['tools']:
        print(t['name'], '|', t['description'][:80])
"
```

### 步驟二：初始化

執行 `init_script_tool.py` 建立目錄與模板：

```bash
python3 scripts/init_script_tool.py <tool-name> --lang <javascript|python>
```

範例：

```bash
python3 scripts/init_script_tool.py fetch_weather --lang javascript
python3 scripts/init_script_tool.py parse_csv --lang python
```

### 步驟三：編輯 tool.json

填入正確的 `description` 與 `parameters` schema。description 決定 agent 何時呼叫此工具，需清楚且完整。

### 步驟四：實作腳本

實作 `script.js` 或 `script.py` 的業務邏輯。若任務需要現有工具，直接使用上方「呼叫現有工具」章節的 `call_tool` / `send` 模板組合，而非重新實作相同邏輯。完成後測試：

```bash
echo '{"city":"Taipei"}' | node ~/.config/agenvoy/script_tools/fetch_weather/script.js
echo '{"city":"Taipei"}' | python3 ~/.config/agenvoy/script_tools/fetch_weather/script.py
```

### 步驟五：重啟 agent

新增 script tool 後需重啟 agent 才會載入。
