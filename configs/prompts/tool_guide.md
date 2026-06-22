# Tool Build Contract

## Choose the right type

| | Script tool | API tool |
|---|---|---|
| When | Multi-step logic, computation on response, non-HTTP, complex auth | Single REST endpoint, no computation, straight request→response |
| Files | tool.json + script.py | single <name>.json |
| Directory | ~/.config/agenvoy/tools/script/<name>/ | ~/.config/agenvoy/tools/api/ |
| Registered as | script_<name> | api_<name> |
| write_tool tags | tag="json" + tag="script" | tag="api" |

## Naming
snake_case, no prefix (runtime adds `script_` or `api_` automatically).
Verb + noun pattern: calculate_rsi, fetch_weather, deduplicate_csv.
Avoid vague verbs: process_*, handle_*.

## Secret / API key access
Never hardcode secrets. Key naming: {BRAND}_API_KEY in SCREAMING_SNAKE_CASE.
Examples: POLYGON_API_KEY, OPENAI_API_KEY, ALPHAVANTAGE_API_KEY.

**Script tools** — read from local keychain endpoint:

    GET http://localhost:17989/v1/key?key=<KEY_NAME>
    → 200 { "value": "<secret>" }
    → 404 or empty value → key not stored

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

**API tools** — set `auth.env` to the keychain key name. Runtime resolves automatically. If not yet stored, call `store_secret`.

---

## Script tool

### How it runs
- Language: Python only (python3)
- Invocation: fork python3 script.py inside OS-native sandbox
- Input: JSON args from stdin (single line); empty args = {}
- Output: stdout must print a single JSON object as tool result
- Error: write stderr + exit non-zero; runtime returns "script error: <stderr>"
- Timeout: 5 minutes hard limit; per-tool override via tool.json "timeout" field (seconds)
- Sandbox: macOS sandbox-exec / Linux bwrap; denies ~/.ssh, ~/.aws, ~/.gcloud, .env, *.pem
- CWD: set by runtime; script should use absolute paths or Path.home()

### tool.json format
```json
{
  "name": "<tool_name>",
  "description": "<when to call this tool — trigger signals and use-case differentiation only, 60-200 chars, no filler>",
  "always_allow": false,
  "concurrent": false,
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
- concurrent: true when the API is read-only data retrieval with a relaxed rate limit; false (default) for write operations or strict rate-limited endpoints.

### script.py template
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

### Implementation rules
- Required params missing → stderr + exit 1
- Single JSON object to stdout; no debug print() to stdout (use stderr)
- Prefer stdlib (json, urllib, csv, re, math, pathlib, datetime) over third-party
- Use absolute paths or Path.home(); never relative paths
- Network requests: timeout ≤ 30s per request, retry ≤ 3
- No writes to sensitive directories (.ssh, .aws, .env)

### Script tool checklist
1. tool.json is valid JSON; every entry in "required" exists in "properties"
2. "name" in tool.json matches the directory name passed to write_tool
3. Script reads stdin JSON as first action: json.loads(sys.stdin.read() or "{}")
4. All success paths output JSON via json.dumps()
5. All error paths write stderr + exit non-zero
6. Every required parameter has a missing-value guard in the script
7. Default values match between tool.json and script code
8. No hardcoded secrets; all credentials via /v1/key endpoint
9. test_tool passed with exit 0 + valid JSON stdout

---

## API tool

### How it runs
- File: <name>.json (single file, no script)
- Invocation: runtime reads the JSON definition, builds and executes the HTTP request
- Input: parameters from the LLM tool call, matched against parameter schema
- Output: raw HTTP response body returned as tool result
- Timeout: 60s default; per-tool override via endpoint.timeout (seconds)
- Auth: credentials resolved from local keychain at runtime

### JSON format
```json
{
  "name": "<tool_name>",
  "description": "<when to call this tool — trigger signals and use-case, 60-200 chars, no filler>",
  "always_allow": false,
  "concurrent": false,
  "endpoint": {
    "url": "https://api.example.com/v1/{resource_id}",
    "method": "GET|POST|PUT|PATCH|DELETE",
    "content_type": "json",
    "headers": {
      "X-Custom-Header": "value"
    },
    "query": {
      "static_param": "value"
    },
    "timeout": 15
  },
  "auth": {
    "type": "bearer|apikey|basic",
    "header": "Authorization",
    "env": "SERVICE_API_KEY"
  },
  "parameters": {
    "<param_name>": {
      "type": "string|integer|number|boolean",
      "description": "<purpose + accepted values + example>",
      "required": true,
      "default": ""
    }
  }
}
```

### Field reference

**endpoint:**
- url: full URL; `{param}` placeholders for path parameters — runtime substitutes from parameters, remaining go to query (GET) or body (POST/PUT/PATCH/DELETE)
- method: GET, POST, PUT, PATCH, or DELETE
- content_type: "json" (default) or "form"
- headers: static headers added to every request
- query: static query parameters added to every GET request
- timeout: seconds; omit for 60s default

**auth** (omit block entirely for public APIs):
- type: "bearer" (Authorization: Bearer), "apikey" (custom header), "basic" (Authorization: Basic)
- header: header name; defaults to "Authorization" for bearer/basic, "X-API-Key" for apikey
- env: keychain key name in SCREAMING_SNAKE_CASE

**parameters:**
- `{name}` in URL = path parameter; runtime substitutes and removes from query/body
- Remaining: query string (GET) or JSON/form body (POST/PUT/PATCH/DELETE)
- required: true = LLM must provide; false = uses default if omitted

### API tool checklist
1. JSON is valid; name is snake_case without api_ prefix
2. endpoint.url is a complete URL with correct {param} placeholders
3. endpoint.method is one of GET/POST/PUT/PATCH/DELETE
4. Every {param} in URL exists in parameters with required=true
5. Auth block present only if API needs credentials; env key is SCREAMING_SNAKE_CASE
6. No hardcoded secrets
7. description is 60-200 chars, trigger-focused, no filler

---

## Execution flow

**Step 1 — Find a suitable API:**
1. `api_public_api_list(type=category)` → pick ≤3 relevant categories → query each
2. Auto-select best candidate: prefer `auth=""` (no key) + `https=Yes`
3. `fetch_page` the candidate's `url` → extract base URL, endpoint, params, response format

**Step 2 — Choose type and create:**

*API tool* (single endpoint, no computation):
4. `write_tool(name, tag="api", content)` = full <name>.json

*Script tool* (multi-step, computation, complex logic):
4a. `write_tool(name, tag="json", content)` = full tool.json
4b. `write_tool(name, tag="script", content)` = full script.py

**Step 3 — Test and fix (script tools only):**
5. `test_tool(name, input)` with JSON string matching the tool's parameters
6. If step 5 fails: `patch_tool(name, tag, old_string, new_string)` → re-test (max 3 retries)

**Step 4 — Answer:**
7. Call the new tool to answer the user's original request

All steps are tool calls. Text output only at the final step. `name` without prefix (runtime adds it). Auth-required APIs: script tools use `get_key()`, API tools set `auth.env` + `store_secret` if key missing.

**Fallback:** if `search_tools` returns no match, or a tool call fails, treat as "no existing tool covers it" and enter this flow. Never say "tool not available" — build one and answer.