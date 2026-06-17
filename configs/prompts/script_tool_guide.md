# Script Tool Contract

## How tools run
- Directory: ~/.config/agenvoy/tools/script/<name>/
- Files: tool.json (schema) + script.py (runtime)
- Registered as: script_<name>
- Language: Python only (python3)
- Invocation: fork python3 script.py inside OS-native sandbox
- Input: JSON args from stdin (single line); empty args = {}
- Output: stdout must print a single JSON object as tool result
- Error: write stderr + exit non-zero; runtime returns "script error: <stderr>"
- Timeout: 5 minutes hard limit; per-tool override via tool.json "timeout" field (seconds)
- Sandbox: macOS sandbox-exec / Linux bwrap; denies ~/.ssh, ~/.aws, ~/.gcloud, .env, *.pem
- CWD: set by runtime; script should use absolute paths or Path.home()

## Naming
snake_case, no script_ prefix (runtime adds it automatically).
Verb + noun pattern: calculate_rsi, fetch_reddit_top, deduplicate_csv.
Avoid vague verbs: process_*, handle_*.

## tool.json format
```json
{
  "name": "<tool_name>",
  "description": "<when to call this tool — trigger signals and use-case differentiation only, 60-200 chars, no filler>",
  "always_allow": false,
  "parameters": {
    "type": "object",
    "properties": {
      "<param_name>": {
        "type": "string|integer|number|boolean|array|object",
        "description": "<purpose + unit + accepted values/enum meanings + example + interactions with other params>",
        "default": "<value, required for optional params>"
      }
    },
    "required": ["<required_param_names>"]
  }
}
```

- description: trigger signals only (when to call, vs similar tools). No implementation details, no filler.
- parameter descriptions: complete contracts. Non-trivial types (object/array/enum) shorter than 20 chars = incomplete.
- always_allow: true for read-only/computation; false for writes/sends/payments.

## Template
```python
#!/usr/bin/env python3
import json
import sys


def main():
    args = json.loads(sys.stdin.read() or "{}")

    symbol = args.get("symbol")
    if not symbol:
        print("missing required parameter: symbol", file=sys.stderr)
        sys.exit(1)

    period = int(args.get("period", 14))

    try:
        result = compute(symbol, period)
    except Exception as e:
        print(f"failed: {e}", file=sys.stderr)
        sys.exit(1)

    print(json.dumps(result))


def compute(symbol, period):
    return {"symbol": symbol, "period": period, "rsi": 50.0}


if __name__ == "__main__":
    main()
```

## Secret / API key access
Never hardcode secrets in script code. Read from the local keychain endpoint:

    GET http://localhost:17989/v1/key?key=<KEY_NAME>
    → 200 { "value": "<secret>" }
    → 404 or empty value → key not stored

Key naming convention: {BRAND}_API_KEY in SCREAMING_SNAKE_CASE.
Examples: POLYGON_API_KEY, OPENAI_API_KEY, ALPHAVANTAGE_API_KEY.

Python helper:
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

## Execution flow

**Step 1 — Find a suitable API:**
1. `api_public_api_list(type=category)` → pick ≤3 relevant categories → query each
2. Auto-select best candidate: prefer `auth=""` (no key) + `https=Yes`
3. `fetch_page` the candidate's `url` → extract base URL, endpoint, params, response format

**Step 2 — Create the script tool (two concurrent `write_tool` calls):**
4a. `write_tool(name, tag="json", content)` = full tool.json (`{"name":"<snake_case>","description":"<60-200 chars>","always_allow":true,"parameters":{...}}`)
4b. `write_tool(name, tag="script", content)` = full script.py (stdin JSON → `urllib.request` → `print(json.dumps(result))` stdout; errors → stderr + `sys.exit(1)`)

**Step 3 — Test and fix:**
5. `test_tool(name, input)` with JSON string matching the tool's parameters
6. If step 5 fails: `patch_tool(name, tag, old_string, new_string)` → re-test (max 3 retries)

**Step 4 — Answer:**
7. Call the new tool (`script_<name>`) to answer the user's original request

All steps (1–7) are tool calls. Text output only at step 7. `name` without `script_` prefix (runtime adds it). Auth-required APIs: add `get_key()` via `http://localhost:17989/v1/key?key=<KEY>` in script + call `store_secret`.

## Implementation rules
- Required params missing → stderr + exit 1
- Single JSON object to stdout; no debug print() to stdout (use stderr)
- Prefer stdlib (json, urllib, csv, re, math, pathlib, datetime) over third-party
- Use absolute paths or Path.home(); never relative paths
- Network requests: timeout ≤ 30s per request, retry ≤ 3
- No writes to sensitive directories (.ssh, .aws, .env)

## Pre-completion checklist
1. tool.json is valid JSON; every entry in "required" exists in "properties"
2. "name" in tool.json matches the directory name passed to write_tool
3. Script reads stdin JSON as first action: json.loads(sys.stdin.read() or "{}")
4. All success paths output JSON via json.dumps()
5. All error paths write stderr + exit non-zero
6. Every required parameter has a missing-value guard in the script
7. Default values match between tool.json and script code
8. No hardcoded secrets; all credentials via /v1/key endpoint
9. test_tool passed with exit 0 + valid JSON stdout