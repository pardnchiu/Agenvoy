### Decide the path

1. **Find an API** — `public_api_list()` → pick ≤3 relevant categories → `public_api_list(category)` each. Every listed API is keyless; prefer `https="Yes"`.
2. **A fitting API exists** — `fetch_page` the candidate's `url` for base URL, endpoint, params and response shape → `http_request` (as many calls as the answer needs) → answer. No tool is built; the same flow repeats next time.
3. **Build a script tool** only outside the public list, when either holds:
   - the user supplies an API that needs a key: a key passed through `http_request` headers lands in the conversation and history, `get_key()` inside the sandbox does not;
   - Python (stdlib, a public data file, a library) can produce the answer.
4. **Nothing fits** — no API and no Python route → `search_web` / `fetch_page`, and say where the data came from.

### Naming
snake_case, no prefix (runtime adds `script_` automatically).
Verb + noun pattern: calculate_rsi, fetch_weather, deduplicate_csv.
Avoid generic verbs: process_*, handle_*, manage_*, execute_*, perform_*, dispatch_*, do_*, run_* (unless the verb IS the literal action, e.g. run_command).
If a same-domain tool already exists, match its verb and suffix shape — don't mix analyze_* into a fetch_* cluster, don't invent a new suffix vocabulary for the same concept (read_error / remember_error / search_error_memory would be three shapes for one idea — pick one).

### Description
Exactly 3 lines, 60-200 chars total. No filler ("This tool allows you to...", "Use this when needed..."), no **bold**, no output-schema dump, no implementation detail (call-contract stuff belongs in parameter descriptions, not here).
1. What — core action, one sentence.
2. When — trigger vs alternatives (`use for X; Y for Z`).
3. Precondition — key constraint/prerequisite the caller must satisfy (omit line if none).
Example:
```
Fetch the current spot price for a crypto symbol from a public exchange API.
Use when the user asks for a live crypto price; use fetch_ohlc for historical candles.
Symbol must be a valid exchange ticker (e.g. BTCUSDT).
```

### Parameter description
Every `parameters.properties.<name>.description` must cover:
- How — type, unit, accepted values/enum meanings.
- When — interaction with other params (omit if the param stands alone).
- Example — at least one concrete value for non-trivial types (object, array, enum, path, cron, regex).
A non-trivial type with a description under 20 chars is treated as incomplete. Don't repeat the field name as the description ("user_id: the user id" is not a description).

### Secret / API key access
Never hardcode secrets. Key naming: {BRAND}_API_KEY in SCREAMING_SNAKE_CASE.
Examples: POLYGON_API_KEY, OPENAI_API_KEY, ALPHAVANTAGE_API_KEY.

Lookup order, same for every tool: OS keychain (service `agenvoy`, account = key name) first, then `~/.config/agenvoy/.secrets` on Linux, then the environment variable of the same name. Nothing injects the key into the script's environment, so a script tool reads it itself.

Read the keychain in-process:

```python
def get_key(name):
    import os, subprocess, sys, pathlib
    if sys.platform == "darwin":
        r = subprocess.run(["security", "find-generic-password", "-s", "agenvoy", "-a", name, "-w"],
                           capture_output=True, text=True)
        if r.returncode == 0 and r.stdout.strip():
            return r.stdout.strip()
    else:
        r = subprocess.run(["secret-tool", "lookup", "service", "agenvoy", "account", name],
                           capture_output=True, text=True)
        if r.returncode == 0 and r.stdout.strip():
            return r.stdout.strip()
        p = pathlib.Path.home() / ".config/agenvoy/.secrets"
        if p.exists():
            for line in p.read_text().splitlines():
                if line.startswith(name + "="):
                    return line[len(name) + 1:]
    val = os.environ.get(name, "")
    if not val:
        raise RuntimeError(f"missing key: {name}")
    return val
```

A key that is not there → `store_secret`, then call the tool again.

---

### Script tool

#### How it runs
- Language: Python only (python3)
- Invocation: fork python3 script.py inside OS-native sandbox
- Input: JSON args from stdin (single line); empty args = {}
- Output: stdout must print a single JSON object as tool result
- Error: write stderr + exit non-zero; runtime returns "script error: <stderr>"
- Timeout: 5 minutes hard limit; per-tool override via tool.json "timeout" field (seconds)
- Sandbox: macOS sandbox-exec / Linux bwrap; denies ~/.ssh, ~/.aws, ~/.gcloud, .env, *.pem
- CWD: set by runtime; script should use absolute paths or Path.home()

#### tool.json format
```json
{
  "name": "<tool_name>",
  "description": "<3-line What/When/Precondition per the Description section above>",
  "always_allow": false,
  "concurrent": false,
  "parameters": {
    "type": "object",
    "properties": {
      "<param_name>": {
        "type": "string|integer|number|boolean|array|object",
        "description": "<How/When/Example per the Parameter description section above>",
        "default": "<value, required for optional params>"
      }
    },
    "required": ["<required_param_names>"]
  }
}
```

- always_allow: true for read-only/computation; false for writes/sends/payments.
- concurrent: true when the API is read-only data retrieval with a relaxed rate limit; false (default) for write operations or strict rate-limited endpoints.

#### script.py template
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

#### Implementation rules
- Required params missing → stderr + exit 1
- Single JSON object to stdout; no debug print() to stdout (use stderr)
- Prefer stdlib (json, urllib, csv, re, math, pathlib, datetime) over third-party
- Use absolute paths or Path.home(); never relative paths
- Network requests: timeout ≤ 30s per request, retry ≤ 3
- No writes to sensitive directories (.ssh, .aws, .env)

#### Script tool checklist
1. tool.json is valid JSON; every entry in "required" exists in "properties"
2. "name" in tool.json matches the directory name passed to edit_tool(mode=write)
3. Script reads stdin JSON as first action: json.loads(sys.stdin.read() or "{}")
4. All success paths output JSON via json.dumps()
5. All error paths write stderr + exit non-zero
6. Every required parameter has a missing-value guard in the script
7. Default values match between tool.json and script code
8. No hardcoded secrets; all credentials via get_key()
9. test_tool passed with exit 0 + valid JSON stdout

---

### Execution flow (script tool)

1. `edit_tool(mode=write, name, tag="json", content)` = full tool.json
2. `edit_tool(mode=write, name, tag="script", content)` = full script.py
3. `test_tool(name, input)` with a JSON string matching the tool's parameters
4. On failure: `edit_tool(mode=patch, name, tag, old_string, new_string)` → re-test (max 3 retries)
5. Call the new tool to answer the user's original request

All steps are tool calls. Text output only at the final step. `name` without prefix (runtime adds it). A key the API needs: `get_key()` in the script, `store_secret` when it is missing.

---

### When not to build

One-off verification, testing or debugging — "測試 X 能不能用", "確認 Y 有沒有生效", probing an endpoint, a parameter or a flag — produces a finding for this turn, not a reusable capability. Write the script inline and run it with `run_command` (`python3 -c`, a heredoc, `curl`); report what came back. No `edit_tool`, no `test_tool`, no saved tool.

The build flow above starts only when Decide the path lands on step 3 and the user will want the data again.

---

**Fallback:** on a data request, if `find_tools(mode=search)` returns no match, or a tool call fails, treat it as "no existing tool covers it" and start from Decide the path. Never stop at "tool not available" — call the API, build a script tool, or fall back to search.