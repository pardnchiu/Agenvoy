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

## 建立流程

### 步驟一：初始化

執行 `init_script_tool.py` 建立目錄與模板：

```bash
python3 scripts/init_script_tool.py <tool-name> --lang <javascript|python>
```

範例：

```bash
python3 scripts/init_script_tool.py fetch_weather --lang javascript
python3 scripts/init_script_tool.py parse_csv --lang python
```

### 步驟二：編輯 tool.json

填入正確的 `description` 與 `parameters` schema。description 決定 agent 何時呼叫此工具，需清楚且完整。

### 步驟三：實作腳本

實作 `script.js` 或 `script.py` 的業務邏輯。完成後測試：

```bash
echo '{"city":"Taipei"}' | node ~/.config/agenvoy/script_tools/fetch_weather/script.js
echo '{"city":"Taipei"}' | python3 ~/.config/agenvoy/script_tools/fetch_weather/script.py
```

### 步驟四：重啟 agent

新增 script tool 後需重啟 agent 才會載入。
