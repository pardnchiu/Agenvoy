---
name: script-tool-add
description: Design and scaffold a Python (or JavaScript) script tool for Agenvoy, writing `tool.json` + `script.py`/`script.js` pair under `~/.config/agenvoy/tools/script/<tool_name>/`. Triggers on requests like "add a script tool", "幫我寫一個 python tool", "做一個 script tool 給 agent 用", "新增腳本工具". Handles parameter schema design, stdlib-vs-third-party dependency check, keychain secret access via local `/v1/key` endpoint, sandbox awareness, and test execution before write.
---

# Script Tool Adder

設計 Python（或 JavaScript）腳本工具，落地至 `~/.config/agenvoy/tools/script/<tool_name>/`，每個 tool 一個目錄含 `tool.json` + `script.py`/`script.js` 配對。

---

## 目標

| 項目 | 說明 |
|---|---|
| **輸入** | 使用者描述「想要這個工具做什麼」——可以是純文字需求、現有 script 改造、或 API 呼叫包裝 |
| **輸出** | `~/.config/agenvoy/tools/script/<tool_name>/{tool.json, script.py}` 配對檔 |
| **語言** | 預設 Python（`script.py` → `python3`）；JavaScript 可選（`script.js` → `node`） |

---

## 執行模型（必先理解）

| 項目 | 規格 |
|---|---|
| **載入** | runtime scan `~/.config/agenvoy/tools/script/` 各子目錄，每個含 `tool.json` 視為一個 tool；註冊名 `script_<name>` |
| **呼叫** | LLM 觸發 → runtime fork `python3 <script.py>`（或 `node <script.js>`）via `go-pkg/sandbox` |
| **輸入** | JSON args 由 **stdin** 餵入（單行）；空 args 為 `{}` |
| **輸出** | **stdout** 必須印單一 JSON 物件作為 tool result；非 JSON 字串 LLM 仍可讀但結構化能力下降 |
| **錯誤** | 寫 **stderr** + `exit non-zero`；runtime 回 LLM 為 `script error: <stderr>` |
| **Timeout** | 硬上限 5 分鐘；超過 runtime kill subprocess |
| **Sandbox** | macOS `sandbox-exec` / Linux `bwrap`；deny `~/.ssh`、`~/.aws`、`~/.gcloud`、`.env`、`*.pem` 等敏感路徑 |
| **CWD** | runtime 帶當前 `WorkDir`；script 應走 absolute path 或 `Path.home()` |

理解此模型是設計 script 的前提——所有 schema 與實作都圍繞「stdin JSON → stdout JSON」契約。

---

## 互動流程（七關卡）

### Gate 1：需求釐清

User message 已含明確需求（例：「寫一個算 RSI 的 tool，輸入 ticker 與 period，輸出 RSI 數值」）→ **跳過** Gate 1。

否則用 `AskUserQuestion` 三題一次問完：

```
Q1 question: "這個 tool 要做什麼？一句話描述。"
   header: "Intent"
   free-text

Q2 question: "輸入是什麼？輸出是什麼？"
   header: "I/O shape"
   free-text（範例：「輸入 ticker (string), period (int)；輸出 {rsi: float, signal: string}」）

Q3 question: "語言偏好？"
   header: "Language"
   options:
     - label: "Python（預設）"  description: "runtime: python3，stdlib 豐富、適合資料處理"
     - label: "JavaScript"      description: "runtime: node，適合輕量 fetch / JSON 處理"
   multiSelect: false
```

### Gate 2：命名與參數設計

#### 命名

`<tool_name>`：snake_case，**不**加 `script_` 前綴（runtime 自動補）。直白具體：動詞+名詞，例：`calculate_rsi`、`deduplicate_csv`、`fetch_reddit_top`。避免動詞模糊（`process_*`／`handle_*`）或名詞前置造成 LLM 選擇困難（`rsi_calc` ≺ `calculate_rsi`，動詞前置與其他 tool 對齊）。

#### 工具描述（`tool.json` 頂層 `description`）

英文。**只描述使用情境**（何時呼叫／與相似 tool 的取捨），**極致精簡精準**——一兩句寫清觸發信號即停。lazy-schema 下這是 LLM 召喚 tool 的唯一依據，但冗詞稀釋訊號；禁填充語、禁實作八卦、禁呼叫合約細節（型別／enum／邊界丟 `parameters`）。長度建議 60-200 chars；超過兩三句通常代表夾雜了該住 schema 的內容。

#### 參數 schema

`tool.json` 的 `parameters` 採 JSON Schema：

| 必須欄位 | 規則 |
|---|---|
| `type` | 永遠 `"object"` |
| `properties` | 每個參數一個 entry |
| `required` | 必填參數 name array |

每個 property：

| 欄位 | 必填 | 規則 |
|---|---|---|
| `type` | ✅ | `string`／`integer`／`number`／`boolean`／`array`／`object` |
| `description` | ✅ | 英文。完整呼叫合約：用途 + 型別與單位（秒／毫秒、bytes／MiB）+ 接受值（enum 含每值意涵、regex、值域）+ 至少一個範例（非平凡型別必給）+ 與其他參數互動 + 邊界。非平凡型別（`object`／`array`／含 `enum`）短於 20 chars 視為不完整 |
| `default` | optional | 非必填參數**必給**；型別需匹配 `type`；缺 default LLM 不知道省略此參數的語意 |
| `enum` | optional | 限制可選值；每個 enum value 在 description 內解釋其意涵 |

**為何 schema description 要這麼完整**：schema 按需注入（非 `AlwaysLoad` 的 tool 預設帶 stub schema）；一旦注入後 LLM 立即基於 description 決定如何填值。缺範例／單位／互動關係 → trial-and-error → 失敗訊息浪費 token。

#### 詢問規則

從 Gate 1 的 I/O 描述抽出參數後，用 `AskUserQuestion` 跟使用者確認 schema：

```
question: "確認以下參數 schema？"
header: "Schema"
（在 Q 文字中列出 draft schema table）
options:
  - label: "確認"           description: "schema 正確，繼續"
  - label: "修改"           description: "free-text 補充修正內容"
multiSelect: false
```

選「修改」→ 進一步 free-text 取修正描述。

### Gate 3：相依管理

#### 偵測

LLM 預判實作需要的 import：

| 類別 | 範例 | 處理 |
|---|---|---|
| **stdlib only** | `json`、`urllib`、`pathlib`、`datetime`、`math`、`csv`、`re`、`hashlib` | ✅ 無需安裝 |
| **常見第三方** | `requests`、`numpy`、`pandas`、`beautifulsoup4`、`yfinance` | ⚠️ 需使用者已安裝 |
| **罕見／重型** | `tensorflow`、`torch`、`opencv-python` | ❌ 不建議；改寫為 stdlib 或請使用者三思 |

#### 詢問

需要第三方時，用 `AskUserQuestion`：

```
question: "此 tool 需要 <package_list>，採哪種策略？"
header: "Dependencies"
options:
  - label: "改用 stdlib 重寫"           description: "盡量純 stdlib，避免外部依賴（推薦）"
  - label: "保留並提醒使用者 pip install" description: "保留第三方，最後輸出提醒安裝命令"
  - label: "取消，重新規劃"             description: "退回 Gate 1 重新設計需求"
multiSelect: false
```

**為何優先 stdlib**：script 在 sandbox 子進程跑，使用者環境的第三方套件版本不可控；stdlib 為唯一保證可用版本。例：純 `urllib` 替代 `requests`、純 `csv` 替代 `pandas.read_csv`。

### Gate 4：Secret／API Key

#### 偵測

實作需要 token／API key／secret 時 — **不**直接讓使用者貼明文到參數，走 keychain。

#### Keychain 存取契約

Agenvoy daemon 啟動時開 HTTP server（預設 `localhost:17989`），暴露：

```
GET http://localhost:17989/v1/key?key=<key_name>
→ 200 OK { "value": "<secret>" }
→ 404 / empty value → key 不存在
```

`<key_name>` 命名慣例：**`{品牌}_API_KEY`**（SCREAMING_SNAKE_CASE，與 providers 同 keychain pool），例 `OPENAI_API_KEY`、`CODEX_API_KEY`、`POLYGON_API_KEY`。底層儲存位置：macOS keychain 中 **service = `agenvoy`**、**account = key 名**，組合識別 `agenvoy.{key}`（例 `agenvoy.OPENAI_API_KEY`）；`/v1/key?key=` 參數**只填 key 名**，不帶 `agenvoy.` 前綴。

Script 端範本（Python）：

```python
def get_key(name):
    import json, urllib.request
    url = f"http://localhost:17989/v1/key?key={name}"
    try:
        with urllib.request.urlopen(url, timeout=5) as r:
            val = json.loads(r.read().decode()).get("value", "")
    except Exception:
        val = ""
    if not val:
        raise RuntimeError(f"missing key: {name}")
    return val
```

#### 詢問

需要 secret 時用 `AskUserQuestion`：

```
question: "此 tool 需要哪個 secret？keychain key 名稱用什麼？"
header: "Secret"
options:
  - label: "<推薦命名>" description: "格式 {品牌}_API_KEY；例 OPENAI_API_KEY、POLYGON_API_KEY、STAGING_API_KEY"
  - label: "自訂"      description: "free-text 輸入"
multiSelect: false
```

取得 key 名後**立即主動呼叫 `store_secret`** 把值落 keychain（不延後到使用者自己跑）：

```
store_secret({
  "key": "<KEYCHAIN_KEY_NAME>",
  "prompt": "請輸入 <tool 名稱> 用的 <secret 用途> 值"
})
```

`store_secret` 內部走 `ask_user(secret:true)` 遮罩輸入後 `keychain.Set` 落地，**skill 全程不見明文**。完成後 Gate 6 試跑 script 即可透過 `/v1/key?key=<KEYCHAIN_KEY_NAME>` 取得真實 value。

**禁止**：(a) 走 `ask_user` 取 plaintext 再轉手 `store_secret`（value 會落 LLM context／history／action.log）；(b) 在 `tool.json.parameters` 暴露 secret 欄位讓 LLM 收 plaintext；(c) script 內 hardcode key 值。schema 只記**取值方式**（`/v1/key` 端點 + key 名），不記值。

### Gate 5：實作

#### Script 樣板（Python）

```python
#!/usr/bin/env python3
import json
import sys


def main():
    args = json.loads(sys.stdin.read() or "{}")

    # 1. 取必填參數，缺則 error
    symbol = args.get("symbol")
    if not symbol:
        print("missing required parameter: symbol", file=sys.stderr)
        sys.exit(1)

    # 2. 取選填參數帶 default（與 tool.json 對齊）
    period = int(args.get("period", 14))

    # 3. 業務邏輯
    try:
        result = compute_rsi(symbol, period)
    except Exception as e:
        print(f"compute failed: {e}", file=sys.stderr)
        sys.exit(1)

    # 4. 單一 JSON 輸出
    print(json.dumps(result))


def compute_rsi(symbol, period):
    # ... 實作 ...
    return {"symbol": symbol, "period": period, "rsi": 50.0}


if __name__ == "__main__":
    main()
```

#### Script 樣板（JavaScript）

```javascript
#!/usr/bin/env node
const fs = require('fs');

function readStdin() {
  const data = fs.readFileSync(0, 'utf-8');
  return data ? JSON.parse(data) : {};
}

function main() {
  const args = readStdin();

  const symbol = args.symbol;
  if (!symbol) {
    process.stderr.write('missing required parameter: symbol\n');
    process.exit(1);
  }

  const period = args.period ?? 14;

  try {
    const result = { symbol, period, rsi: 50.0 };
    process.stdout.write(JSON.stringify(result));
  } catch (e) {
    process.stderr.write(`compute failed: ${e.message}\n`);
    process.exit(1);
  }
}

main();
```

#### 實作規則

| 規則 | 為何 |
|---|---|
| **必填參數缺漏 → stderr + exit 1** | runtime 將 stderr 包成 LLM 可讀錯誤；exit 0 + 空輸出會讓 LLM 困惑 |
| **單一 JSON stdout** | LLM 預期結構化輸出；多行輸出仍可讀但解析難度高 |
| **stdlib 為主** | 第三方在 sandbox 子進程版本不可控 |
| **絕對路徑或 `Path.home()`** | CWD 由 runtime 決定；相對路徑可能讀不到預期檔案 |
| **不寫進敏感目錄** | sandbox 已 deny `.ssh`／`.aws`／`.env` 等，但 script 應主動避開避免被 sandbox 截 |
| **網路請求帶 timeout** | 預設 5 分鐘總 timeout，個別 request 建議 ≤ 30s 並 retry 限 3 次 |
| **不 `print()` debug 到 stdout** | 污染 JSON 輸出；debug 走 stderr 或不打 |
| **避免長時間 sleep** | timeout 5 分鐘扣除掉就無實際工作時間 |

### Gate 6：試跑驗證（**必跑、未通過禁止寫入**）

#### 試跑前準備

1. 在 `extensions/scripts/.scratch/<tool_name>/`（臨時）寫出 draft `tool.json` + `script.py`——**或**直接寫到目標路徑 `~/.config/agenvoy/tools/script/<tool_name>/` 並標記「draft」
2. 取樣值：從 schema `default` 或 `description` 抽範例值；缺則用 `AskUserQuestion` 詢問每個 required 的測試值

#### 試跑執行

用 `run_command` 跑：

```bash
echo '<sample_json_args>' | python3 ~/.config/agenvoy/tools/script/<tool_name>/script.py
```

捕 stdout / stderr / exit code。

#### 結果判定

| 狀態 | 判定 | 行為 |
|---|---|---|
| exit 0 + stdout 為合法 JSON | ✅ 通過 | 進入 Gate 7 |
| exit 0 + stdout 非 JSON | ⚠️ 結構化失敗 | 顯示 stdout → 詢問是否補 `json.dumps` 重試 |
| exit 0 + stdout 為空 | ⚠️ 無輸出 | 顯示警告 → 詢問是否補輸出邏輯重試 |
| exit ≠ 0 + stderr 有錯誤訊息 | ❌ 邏輯錯 | 顯示 stderr → 修正後重試（最多 3 輪） |
| timeout（>5min）| ❌ 卡住 | kill → 檢查無限迴圈／網路 timeout 缺失 |
| `python3: command not found` | ❌ runtime 缺 | 提示使用者裝 Python 3 後重試 |
| ImportError | ❌ 第三方缺 | 回 Gate 3 重議：改 stdlib 或請使用者裝 |

未通過 → **拒絕寫入正式路徑**。

### Gate 7：always_allow 設定

決定 `tool.json` 頂層 `always_allow` 旗標。控制 `agen cli` 互動模式下是否跳過 confirm prompt——`true` = 不問直接執行、缺省／`false` = 每次 confirm。

#### 預設推薦

| 條件 | 預設建議 | 理由 |
|---|---|---|
| 純讀取／計算（無檔案寫入、無對外發送） | `true` | 無副作用 |
| 唯讀外部 API（GET only） | `true` | 純資料取得 |
| 寫檔（local file save）| `false` | 有副作用，使用者應每次明示位置 |
| 對外發送（mail／webhook／post）| `false` | 一發出無法收回 |
| 涉及金流／支付 | `false` | 強制每次 confirm |
| 修改使用者資料／設定 | `false` | 持久性變更 |

#### 詢問

```
question: "<tool_name> 是否設為 always_allow？建議：<true|false>，理由：<推薦理由>"
header: "Auto-allow"
options:
  - label: "是，跳過每次 confirm"  description: "agen cli 互動模式直接執行"
  - label: "否，每次 confirm"      description: "agen cli 互動模式每次跳出確認 prompt"
multiSelect: false
```

#### 規則

- 寫入 schema 頂層 `always_allow: <bool>`
- 預設不寫此欄位（缺省 = `false`）；只有使用者明確選 `true` 才寫入
- 寫入類／發送類 script 即使使用者選 `true` 也須二次確認該風險

---

## 輸出格式（嚴格遵守）

### `tool.json`

```json
{
  "name": "calculate_rsi",
  "description": "Compute the Relative Strength Index (RSI) momentum oscillator for a given ticker over N trading periods. Use when the user asks 'is X overbought/oversold', 'compute RSI for Y', or mentions any momentum / mean-reversion analysis. RSI > 70 typically signals overbought, < 30 oversold — the tool returns the raw number plus a derived signal label. Pair with fetch_yahoo_finance when you need the underlying OHLCV data first.",
  "always_allow": true,
  "parameters": {
    "type": "object",
    "properties": {
      "symbol": {
        "type": "string",
        "description": "Ticker symbol in Yahoo Finance format (e.g. \"AAPL\" for stocks, \"BTC-USD\" for crypto, \"^GSPC\" for indices). Case-insensitive but uppercase is preferred. Must be a single ticker — for batch use, call the tool repeatedly."
      },
      "period": {
        "type": "integer",
        "description": "RSI lookback window in trading days. Integer between 2 and 200 (typical: 14 for daily, 9 for short-term, 25 for swing trading). Larger periods smooth the signal but lag price action.",
        "default": 14
      }
    },
    "required": ["symbol"]
  }
}
```

### `script.py` / `script.js`

依 Gate 5 樣板實作。**必須**：
- 從 stdin 讀 JSON args
- 從 stdout 印單一 JSON 結果
- error 走 stderr + exit non-zero

---

## 寫入規則

### 路徑

```
~/.config/agenvoy/tools/script/<tool_name>/
├── tool.json
└── script.py    # 或 script.js
```

`<tool_name>` 為 schema 內的 `name`（snake_case，不加 `script_` 前綴）。

### 寫前檢查

| 條件 | 行為 |
|---|---|
| 目錄不存在 | `run_command` 跑 `mkdir -p ~/.config/agenvoy/tools/script/<tool_name>` |
| 同名目錄已存在 | `read_file` 讀現有 `tool.json` 比對；不一致 → `AskUserQuestion`「覆蓋／改名／略過」 |

### 寫入方式

| 檔案 | 工具 | 注意 |
|---|---|---|
| `tool.json` | `write_file` | absolute path，pretty-printed JSON（兩空格縮排、`\n` 結尾） |
| `script.py` / `script.js` | `write_file` | absolute path，`#!/usr/bin/env python3`（或 node）首行，UTF-8 |

寫入後 `run_command` 跑 `chmod +x` 非必須（runtime 用 `python3 <path>` 不靠 shebang）。

---

## 完成回報

每個 tool 寫入後輸出：

```
✅ <tool_name> → ~/.config/agenvoy/tools/script/<tool_name>/
   language: <python|javascript>
   params: <required>/<total>  secret: <key_name|none>  auto-allow: <yes|no>
   試跑: <sample_args> → <truncated_stdout_preview>
```

最後總結：

```
Wrote 1 script tool to ~/.config/agenvoy/tools/script/
重啟 agen daemon（`agen stop && agen`）即可載入。
```

Gate 4 涉及 secret 的 tool，keychain 已於該關卡透過 `store_secret` 落地，無須使用者額外動作。

---

## 反幻覺檢查（產出前必驗）

1. **`tool.json` 合法**：合法 JSON、無尾逗號、`type=object`、`required` array 內每項都存在於 `properties`
2. **`name` 對齊**：`tool.json.name` 與目錄名一致、與檔內 stdin 讀取行為對齊
3. **Stdin 契約**：script 第一段必為 `json.loads(sys.stdin.read() or "{}")`（或 JS 等價）
4. **Stdout 契約**：所有成功 path 走 `print(json.dumps(...))`（或 `process.stdout.write(JSON.stringify(...))`），無多餘 `print()` debug
5. **Stderr 契約**：error path 走 `print(..., file=sys.stderr); sys.exit(1)`
6. **必填參數防護**：tool.json `required` 內每個參數，script 端有缺值檢查 + stderr exit
7. **Default 對齊**：tool.json `default` 值與 script `args.get(name, default)` 內 default 一致
8. **依賴可解**：第三方套件已通過 Gate 3 確認；stdlib 解決方案優先採用
9. **Secret 不留明文**：所有 secret 透過 `/v1/key` 取得，不寫死、不收 plaintext 參數
10. **試跑通過**：Gate 6 取得 exit 0 + 合法 JSON stdout（或使用者明確接受的非 JSON 輸出）
11. **always_allow 確認**：Gate 7 已決定；寫入類／發送類 script 即使選 `true` 已二次確認
12. **Description 極致精簡精準**：`tool.json.description` 只描述使用情境（何時用／與相似 tool 的取捨），一兩句寫清觸發信號即停。純「執行什麼」一句話必失敗（trigger coverage 不足）；夾雜實作細節／呼叫合約／填充語也必失敗（冗詞稀釋訊號）。長度 60-200 chars。
13. **Parameter description 完整**：每個 `properties[*].description` 含型別／單位／接受值／範例／互動關係。非平凡型別（`object`／`array`／含 `enum`）短於 20 chars 必失敗。

---

## 範例：完整一次互動

User: `幫我寫一個 tool，給 ticker 算 RSI`

1. **Gate 1**：需求已明確（ticker → RSI），用 `AskUserQuestion` 補問 I/O shape 與語言 → 使用者答「輸入 symbol+period，輸出 rsi 數值；Python」
2. **Gate 2**：命名 `rsi_calc` → schema 草稿（`symbol: string required`、`period: integer default 14`）→ 使用者確認
3. **Gate 3**：判定純 stdlib（用 `urllib` 抓 Yahoo Finance）→ 無需詢問
4. **Gate 4**：Yahoo Finance 免 token → 無 secret 需求，跳過
5. **Gate 5**：產 `script.py`（含 `urllib.request` fetch、RSI 計算、JSON 輸出）+ `tool.json`
6. **Gate 6**：`echo '{"symbol":"AAPL"}' | python3 script.py` → `{"symbol":"AAPL","period":14,"rsi":52.3}` ✅
7. **Gate 7**：純讀取無副作用 → 建議 `always_allow=true` → 使用者確認
8. **寫入**：`~/.config/agenvoy/tools/script/rsi_calc/{tool.json, script.py}`
9. **回報**：列出路徑 + 試跑結果 + 提醒重啟 daemon

---

## 參考

- Runtime 載入點：`internal/tools/executor.go` `scriptToolbox.Scan(filesystem.ScriptToolsDir)`
- 執行實作：`internal/toolAdapter/script/ececute.go`（5min timeout、sandbox wrap、stdin JSON、stdout result）
- Schema 型別：`internal/toolAdapter/script/translator.go` `ScriptDoc`
- 內建範例：`extensions/scripts/{gex-analyze, smile-analyze}/`
- Keychain 端點：`internal/routes/handler/keyHandler.go`（`GET /v1/key`）
- Sandbox 政策：`configs/jsons/denied_map.json`
