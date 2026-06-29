# Tool Extension

Agenvoy supports four ways to add new tools beyond the built-in set: auto-generation from a capability gap, script tools, API tools, and MCP tools.

## Auto-generation (Capability Gap)

When a user request needs live external data (weather, currency, stock, geocoding, translation, etc.) and no existing tool covers it, the agent creates the tool on the spot, then runs it to answer. No programming required.

The system prompt `Capability Gap` section drives the sequence:

| Step | Action |
|---|---|
| 1. Find a suitable API | `api_public_api_list(type=category)` to pick relevant categories, select best candidate (prefer no-auth + HTTPS), then `fetch_page` the docs |
| 2. Create the script tool | `mkdir` the tool dir, then `write_file` a `tool.json` (name, description, schema) + `script.py` (stdin JSON to HTTP call to stdout JSON) |
| 3. Run and answer | Pipe the user query into the new script; if it fails, fix and retry (max 3) |

After creation, the tool persists at `~/.config/agenvoy/tools/script/<name>/` and is available in all future sessions. Auth-required APIs are handled via `store_secret` + keychain integration in the generated script.

Key constraints:
- The agent must never answer with raw `send_http_request` or inline `python3 -c`; it must write a reusable script to disk
- `fetch_page` is allowed only for reading API documentation, not for fetching answer data
- The generated `tool.json` uses `"always_allow": true` so the tool runs without confirmation on subsequent calls

## Script tools (`script_*`)

Drop a Python / Node.js / shell script under `extensions/scripts/<name>/` along with a `tool.json` descriptor. Agenvoy auto-registers it as `script_<name>` at startup.

```
extensions/scripts/my-tool/
├── tool.json     # name, description, parameter schema, command
└── run.py        # actual script
```

## API tools (`api_*`)

Drop a JSON file under `extensions/apis/<name>.json` describing a REST endpoint. It auto-registers as `api_<name>`. Each `api_<name>` has its own per-name 1 s rate limiter (`reserveAPISlot`).

**Confirm gate** --- `api_*` tools are not prefix-exempt from confirmation. Users may define destructive endpoints (DELETE / POST writes), so `agen cli` confirms each call. Use `agen run` for batch auto-approval.

## MCP tools (`mcp__*`)

Tools exposed by an MCP server are auto-registered as `mcp__<server>__<tool>`. MCP tool output is capped at **1 MiB** per call to keep tool results within provider limits.
