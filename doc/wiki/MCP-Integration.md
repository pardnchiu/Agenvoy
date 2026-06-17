# MCP Integration

> [中文](MCP-Integration.zh.md)

Agenvoy is both an MCP **client** (connects to external MCP servers) and an MCP **server** (exposes its sandboxed tool layer to external agents).

## MCP Server — universal sandbox tool layer for external agents

When launched via stdio pipe, `agen` runs as an MCP server. Any MCP-compatible agent (Claude Code, Codex, OpenCode, etc.) can connect and use Agenvoy's tools — including building new ones on the fly.

### What external agents gain

- **Sandboxed execution** — all script tools run inside OS-native sandbox (macOS `sandbox-exec` / Linux `bwrap`), isolating `~/.ssh`, `~/.aws`, `.env`, `*.pem` and other sensitive paths
- **Auto tool creation** — when no existing tool covers a request, the agent calls `script_tool_generate_guide` to get the build contract, then `write_tool` → `test_tool` to create a new Python script tool. The tool is persisted and reusable across sessions
- **Shared tool library** — tools created by any agent (Agenvoy internal, Claude Code, Codex, etc.) are saved to `~/.config/agenvoy/tools/script/` and available to all connected agents. Build once, use everywhere
- **Live data access** — `api_public_api_list` indexes free public APIs; the agent picks one, scaffolds a script tool around it, and answers with real data instead of training-knowledge guesses

### Quick setup

TUI: `/mcp install` → select your agent

Manual config per agent:

**Claude Code** — `~/.claude.json`
```json
{ "mcpServers": { "agenvoy": { "command": "agen" } } }
```

**Codex** — `~/.codex/config.toml`
```toml
[mcp_servers.agenvoy]
command = "agen"
```

**OpenCode** — `~/.config/opencode/opencode.jsonc`
```json
{ "mcp": { "agenvoy": { "type": "local", "command": ["agen"] } } }
```

### Generic MCP client setup

For any MCP client not listed above, the only requirement is:

- **Transport**: stdio (JSON-RPC over stdin/stdout)
- **Command**: `agen`
- **Args**: none
- **Prerequisite**: `agen` binary in `$PATH` (`curl -fsSL https://cloud.agenvoy.com/install.sh | bash`)

The server speaks [MCP protocol version `2024-11-05`](https://spec.modelcontextprotocol.io/specification/2024-11-05/), supports `tools/list` (with `listChanged` notifications) and `tools/call`. No authentication required — the server runs locally as the current user.

Typical config pattern across MCP clients:

```json
{
  "<servers_key>": {
    "agenvoy": {
      "command": "agen"
    }
  }
}
```

Where `<servers_key>` varies by client (`mcpServers`, `mcp_servers`, `mcp`, etc.). Some clients require an explicit `"type": "stdio"` or `"type": "local"` field. Check your client's documentation.

### Exposed tools

| Tool | Purpose |
|---|---|
| `script_*` / `api_*` / `ext_*` | User-created and extension tools (auto-discovered from disk) |
| `write_tool` | Write tool.json or script.py to a script tool directory |
| `test_tool` | Run a script tool in sandbox with sample input |
| `patch_tool` | String-replace fix inside a tool file |
| `remove_tool` | Move a script tool to trash |
| `list_tools` | List all tools exposed by the server |
| `script_tool_generate_guide` | Return the Script Tool Contract (naming, template, execution flow, checklist) |
| `api_public_api_list` | Browse free public APIs by category for tool creation |

Tool CRUD (`write_tool`, `test_tool`, `patch_tool`, `remove_tool`) are shared with Agenvoy's internal runtime — same handler, same schema, bridged via `toolRegister`. No duplicate implementation.

### Hot reload

The server watches tool directories via `fsnotify`. When a tool is created, modified, or deleted, the server automatically rescans and sends `notifications/tools/list_changed` — the client refreshes its tool list without reconnecting.

---

## MCP Client

The MCP client lets Agenvoy agents call tools exposed by any MCP server.

## Configuration layout

Two layers — the session layer overrides the global layer:

```
~/.config/agenvoy/mcp.json                        ← global
~/.config/agenvoy/sessions/<sid>/mcp.json         ← session-scoped
```

### JSON format

```json
{
  "servers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "${GITHUB_TOKEN}" }
    },
    "remote-api": {
      "url": "https://api.example.com/mcp",
      "headers": { "Authorization": "Bearer ${TOKEN}" }
    }
  }
}
```

`command` and `url` are mutually exclusive. `${VAR}` / `$VAR` placeholders inside `env`, `headers`, and `args` are expanded with `os.Expand` at startup.

## Transport types

| Type | Config keys | Use case |
|---|---|---|
| stdio | `command` + `args` + `env` | Local CLIs (`npx`, native binaries) |
| HTTP + SSE | `url` + `headers` | Remote services, long-running MCP servers |

stdio uses line-delimited JSON-RPC over stdin/stdout. The HTTP transport auto-detects `Content-Type: text/event-stream` per response and falls back to plain JSON otherwise.

## CLI management

```bash
agen mcp list             # List all configured MCP servers (global + per-session)
agen mcp add              # Interactive add via promptui
agen mcp remove           # Interactive remove (with scope label)
```

`agen mcp add` walks through:

1. Server name
2. Type — Local (stdio) / Remote (HTTP)
3. Type-specific fields (command/args/env or url/headers)
4. Scope — Global / pick a session

Scope writes one file only — global writes `~/.config/agenvoy/mcp.json`, session writes the corresponding `~/.config/agenvoy/sessions/<sid>/mcp.json`. No cross-file shuffling.

## Tool naming

MCP-exposed tools are auto-registered with the format:

```
mcp__<server_name>__<tool_name>
```

Example: `mcp__github__create_issue`, `mcp__sqlite-notes__read_query`.

## Result size cap

Each MCP tool result is capped at **1 MiB**. When exceeded, the result is truncated with the marker:

```
[mcp output truncated: <total> bytes total, <kept> kept; consider LIMIT / filter / pagination]
```

This avoids OpenAI Responses API's 10 MB single-tool-output limit triggering a same-signature retry storm. SQLite `SELECT *` on a large table will hit this — add `LIMIT` / `WHERE`.

## Confirm behavior

MCP tools route through the most conservative defaults:

- `agen cli` — confirms each MCP tool call individually
- `agen run` — auto-approves
- No per-server `read_only` toggle — agenvoy does not extend trust to third-party servers because their behavior is unverifiable (a Slack MCP could silently send messages, a Filesystem MCP could silently write files)

For batch operation, use `agen run`. For ad-hoc usage, accept the per-call confirm cost.

## Lifecycle

- **Startup**: `runApp` / `runAgent` calls `mcp.New(ctx, sid)` → `RegisterAll(ctx)` **before** `buildAgentRegistry()` and registers `defer Close()`
- **Per-server failures**: server start failure or `ListTools` failure → `slog.Warn` and skip; never block core functionality
- **Snapshot at start**: the session ID is locked at first resolve; switching sessions requires a restart to reload server lists

## Recommended servers

Zero-auth, locally executed (no API key required):

| Server | Purpose |
|---|---|
| `mcp-server-sqlite` | Run SQL on local `.db` files |
| `@modelcontextprotocol/server-memory` | Persistent knowledge graph |
| `@playwright/mcp` | Browser automation (downloads chromium) |
| `@modelcontextprotocol/server-postgres` | Local Postgres connection |
| `mcp-server-time` | Timezone conversion / relative time |

Avoid registering MCP servers whose capabilities overlap with built-in tools (e.g., `filesystem`, `git`, `fetch`, `shell`) — duplicates only inflate the LLM tool list.

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
