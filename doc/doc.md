# Agenvoy - Documentation

> Back to [README](../README.md)

## Prerequisites

### System Requirements

- Go 1.25 or higher
- At least one AI provider credential (GitHub Copilot subscription or any API key)
- Discord Bot Token (Discord mode only)

### Sandbox Dependencies

| Platform | Dependency | Notes |
|----------|-----------|-------|
| Linux | `bubblewrap` (`bwrap`) | Auto-detected on startup; if missing, installed via `apt-get` / `dnf` / `yum` / `pacman` / `apk` |
| macOS | `sandbox-exec` | Built into macOS, no installation required |

### Browser Dependencies (Optional)

- Chromium or Google Chrome — used by `fetch_page` and `save_page_to_file` in headless mode
- `go-rod` auto-downloads Chromium on first use if not present

### Go Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/bwmarrin/discordgo` | Discord Bot API |
| `github.com/gin-gonic/gin` | REST API server (HTTP routing) |
| `github.com/go-rod/rod` | Headless Chrome browser automation |
| `github.com/go-shiori/go-readability` | HTML content extraction and cleanup |
| `github.com/joho/godotenv` | `.env` environment variable loading |
| `github.com/manifoldco/promptui` | Interactive CLI selection menus |
| `github.com/pardnchiu/ToriiDB` | Embedded KV store (session history, error memory, web caches) |
| `github.com/pardnchiu/go-scheduler` | Cron expression parsing and scheduling |
| `github.com/rivo/tview` | Terminal UI framework |
| `github.com/gdamore/tcell/v2` | Terminal cell and event library |
| `github.com/fsnotify/fsnotify` | Filesystem event monitoring (TUI file watcher) |
| `golang.org/x/net` | HTML tokenizer and network utilities |

## Installation

### Using go install

```bash
go install github.com/pardnchiu/agenvoy/cmd/app@latest
```

### From Source (build + install)

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
make build  # builds as `agen` and installs to /usr/local/bin/agen
```

### Running Directly From Source (no global install)

```bash
make app                # start TUI + Discord + REST API
make run <input...>     # run agent with all tools auto-approved
make cli <input...>     # run agent with per-tool confirmation
```

## Configuration

### Adding a Provider

Run the interactive setup to select a provider and model from the embedded registry:

```bash
agen add
```

Supported providers:

| Provider | Authentication | Default Model |
|----------|---------------|---------------|
| GitHub Copilot | OAuth Device Code Flow (auto-refresh) | `gpt-4.1` |
| OpenAI | API Key (keychain) | `gpt-5-mini` |
| OpenAI Codex | OAuth Device Code Flow (auto-refresh) | `gpt-5.3-codex` |
| Claude | API Key (keychain) | `claude-sonnet-4-5` |
| Gemini | API Key (keychain) | `gemini-2.5-pro` |
| NVIDIA | API Key (keychain) | `openai/gpt-oss-120b` |
| Compat | Optional API Key (keychain) | User-specified |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes (Discord mode) | Discord Bot Token |
| `DISCORD_GUILD_ID` | No | Restricts slash command registration to a specific guild |
| `PORT` | No | REST API server listen port (default: `17989`) |
| `MAX_HISTORY_MESSAGES` | No | Max history messages sent to agent (default: 16) |
| `MAX_TOOL_ITERATIONS` | No | Max tool call iterations per request (default: 16) |
| `MAX_SKILL_ITERATIONS` | No | Max tool call iterations within a skill execution (default: 128) |
| `MAX_EMPTY_RESPONSES` | No | Max consecutive empty responses before giving up (default: 8) |
| `MAX_SESSION_TASKS` | No | Max concurrent tasks per session; over the cap, callers queue (default: 3, hard cap: 10) |
| `MAX_SUBAGENT_TIMEOUT_MIN` | No | `invoke_subagent` execution timeout in minutes — covers slot wait + run (default: 10, hard cap: 60) |
| `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` | No | External CLI agent (codex/claude/copilot/gemini) subprocess timeout in minutes (default: 10, hard cap: 60) |
| `EXTERNAL_COPILOT` | No | Set to `true` to enable the GitHub Copilot CLI external agent |
| `EXTERNAL_CLAUDE` | No | Set to `true` to enable the Claude Code CLI external agent |
| `EXTERNAL_CODEX` | No | Set to `true` to enable the OpenAI Codex CLI external agent |
| `EXTERNAL_GEMINI` | No | Set to `true` to enable the Gemini CLI external agent |
| `OPENAI_API_KEY` | No | Enables semantic indexing for session history and error memory via `text-embedding-3-small` (falls back to keyword scan when unset) |

Create a `.env` file and fill in the values:

```bash
cp .env.example .env
```

> Files named with `.example` (e.g., `.env.example`) bypass the env prefix deny rule and are safe to read.

### API Extensions

Place JSON files in `~/.config/agenvoy/api_tools/` to add custom API tools. Each file defines one callable tool and is loaded at startup:

```json
{
  "name": "my_tool",
  "description": "What the agent sees when selecting this tool",
  "endpoint": {
    "url": "https://api.example.com/resource/{id}",
    "method": "GET",
    "content_type": "json",
    "timeout": 30
  },
  "auth": {
    "type": "bearer",
    "env": "MY_API_KEY"
  },
  "parameters": {
    "id": {
      "type": "string",
      "description": "Resource ID",
      "required": true
    }
  },
  "response": {
    "format": "json"
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Snake_case tool name registered with the agent |
| `description` | Yes | Purpose shown to the LLM for tool selection |
| `endpoint.url` | Yes | Target URL; `{param}` placeholders are substituted at call time |
| `endpoint.method` | Yes | HTTP method: `GET`, `POST`, `PUT`, `DELETE`, `PATCH` |
| `endpoint.content_type` | No | `json` (default) or `form` |
| `endpoint.headers` | No | Static headers map |
| `endpoint.timeout` | No | Request timeout in seconds (default: 30) |
| `auth.type` | No | `bearer` or `apikey` |
| `auth.env` | No | Environment variable name holding the credential |
| `auth.header` | No | Header name for `apikey` type (default: `X-API-Key`) |
| `parameters` | Yes | Flat map of parameter definitions |
| `response.format` | No | `json` (default) or `text` |

Each parameter entry supports: `type` (`string` / `integer` / `number` / `boolean`), `description`, `required`, `default`, and `enum`.

#### Embedded Public API Extensions

The following extensions are bundled and loaded automatically at startup:

| Extension | Category | Description |
|-----------|----------|-------------|
| `nominatim` | Geocoding | OpenStreetMap geocoding and reverse geocoding |
| `coingecko` | Finance | Cryptocurrency prices and market data |
| `wikipedia` | Data | Wikipedia article search and content |
| `world-bank` | Data | World Bank development indicators |
| `usgs-earthquake` | Data | USGS earthquake feed |
| `themealdb` | Data | Recipe and meal database |
| `hackernews` | Data | Hacker News top stories and items |
| `rest-countries` | Data | Country information and metadata |
| `exchange-rate` | Finance | Currency exchange rates |
| `ip-api` | Network | IP geolocation lookup |
| `open-meteo` | Weather | Open-source weather forecast API |
| `youtube` | Media | YouTube video metadata |

### Script Tool Extensions

Place a subdirectory containing `tool.json` + `script.js` or `script.py` in `~/.config/agenvoy/script_tools/` (or `<workdir>/.config/agenvoy/script_tools/`). The executor scans both paths on startup and registers each tool with the `script_` prefix.

#### Bundled Installers

The repository ships ready-to-use script tool extensions with cross-platform install scripts:

```bash
# Install Threads API tools
bash install_threads.sh

# Install yt-dlp tools
bash install_youtube.sh
```

Both scripts detect the OS, verify Python and required packages, and copy the tools to `~/.config/agenvoy/script_tools/`.

| Bundled Tool | Script | Description |
|---|---|---|
| `script_threads_get_quota` | Python | Fetch Threads API usage quota |
| `script_threads_publish_text` | Python | Publish a text post (500-char pre-validation) |
| `script_threads_publish_image` | Python | Publish an image post with caption |
| `script_threads_publish_carousel` | Python | Publish a multi-image carousel post |
| `script_threads_refresh_token` | Python | Refresh a long-lived Threads access token |
| `script_yt_dlp_info` | JS / Python | Fetch video metadata without downloading |
| `script_yt_dlp_downloader` | Python | Download video with NFC filename sanitization |

Script tool directory layout:

```
~/.config/agenvoy/script_tools/
└── my-tool/
    ├── tool.json       # Tool manifest
    └── script.py       # or script.js
```

Script I/O contract — the executor pipes the tool parameters as JSON to stdin and reads the result from stdout:

```python
#!/usr/bin/env python3
import json, sys

params = json.loads(sys.stdin.read() or "{}")
result = {"output": params.get("input", "").upper()}
print(json.dumps(result))
```

### Skill Extensions

Skill extensions are Markdown files with a YAML frontmatter header. On startup, `SyncSkills` copies any skill directories from the embedded `extensions/skills` FS into `~/.config/agenvoy/skills/` if not already present; the scanner then reads 9 standard paths in priority order.

```markdown
---
name: my-skill
description: One-line summary shown to the agent for skill selection
---

# My Skill

Instructions the agent follows when this skill is selected...
```

#### Bundled Skills

The repository ships the following skills under `extensions/skills/`; they are synced to `~/.config/agenvoy/skills/` on first launch:

| Skill | Purpose |
|---|---|
| `code-reviewer` | Review changed source for security / performance / architecture issues; produces a categorized report |
| `commit-generate` | Generate bilingual (English + Traditional Chinese) commit messages from staged changes |
| `readme-generate` | Generate or refresh bilingual `README.md` / `doc/` / `architecture` documentation from project source |
| `schedule-task` | Convert natural-language requests into `add_task` / `add_cron` calls with the right cron expressions |
| `script-tool-creator` | Scaffold a new script-tool extension (`tool.json` + `script.py`/`script.js`) under `~/.config/agenvoy/script_tools/` |
| `skill-creator` | Scaffold a new skill (frontmatter + body) and place it under `~/.config/agenvoy/skills/` |
| `swagger-to-api` | Convert an OpenAPI / Swagger spec into one `extensions/apis/*.json` per endpoint |
| `tool-reviewer` | Audit every registered tool (built-in, API, script) against the naming / description / schema ruleset and produce a violation report |
| `version-generate` | Walk commits since the last tag and emit `.doc/version-generate/vX.Y.Z.md` plus an updated `CHANGELOG.md` index |

## Usage

### Using Make

From the project root:

| Target | Command | Description |
|--------|---------|-------------|
| `make build` | `go build -o agen ./cmd/app/ && sudo mv agen /usr/local/bin/agen` | Build the binary and install to `/usr/local/bin/agen` |
| `make app` | `go run ./cmd/app/` | Start unified app (TUI + Discord + REST API) |
| `make add` | `go run ./cmd/app/ add` | Interactively add a provider/model |
| `make remove` | `go run ./cmd/app/ remove` | Remove a configured provider |
| `make planner` | `go run ./cmd/app/ planner` | Set the planner model |
| `make reasoning` | `go run ./cmd/app/ reasoning` | Set the reasoning level |
| `make models` | `go run ./cmd/app/ list` | List configured models |
| `make skills` | `go run ./cmd/app/ list skill` | List available skills |
| `make cli <input...>` | `go run ./cmd/app/ cli <input>` | Run agent with tool confirmation |
| `make run <input...>` | `go run ./cmd/app/ run <input>` | Run agent with all tools auto-approved |
| `make new [name]` | `go run ./cmd/app/ new [name]` | Create a new `cli-` session and switch to it; `[name]` writes `bot.md` frontmatter `name` (v0.20.0) |
| `make switch <name>` | `go run ./cmd/app/ switch <name>` | Switch the active CLI session by `bot.md` frontmatter `name` (v0.20.0) |
| `make config` | `go run ./cmd/app/ config` | Edit the current CLI session's `bot.md` in `$EDITOR` (v0.20.0) |

### Basic

Start the TUI app (default behavior, no arguments):

```bash
agen
```

List all configured models:

```bash
agen list
```

List all available skills:

```bash
agen list skill
```

Run in interactive mode (confirms each tool call before execution):

```bash
agen cli "analyze the architecture of this project"
```

### Advanced

Auto-approve mode (skip all confirmation prompts):

```bash
agen run "generate and write the README documentation"
```

Remove a provider:

```bash
agen remove
```

Set the planner (router) model:

```bash
agen planner
```

## CLI Reference

### Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| *(none)* | `agen` | Start the unified app (TUI + Discord + REST API) |
| `add` | `agen add` | Interactively register an AI provider |
| `remove` | `agen remove` | Remove a configured provider |
| `planner` | `agen planner` | Set the planner (router) model |
| `reasoning` | `agen reasoning` | Configure reasoning level for a provider |
| `list` | `agen list [skill]` | List configured models or available skills |
| `cli` | `agen cli <input...>` | Execute agentic workflow with interactive confirmation |
| `run` | `agen run <input...>` | Execute with all tool calls auto-approved |
| `new` | `agen new [name]` | Create a new `cli-` session and switch to it; `[name]` is written to `bot.md` frontmatter (v0.20.0) |
| `switch` | `agen switch <name>` | Resolve `<name>` against `cli-*`/`http-*` `bot.md` frontmatter and switch the active CLI session (v0.20.0) |
| `config` | `agen config` | Open the current CLI session's `bot.md` in `$EDITOR` (default: `vi`) (v0.20.0) |

### TUI Keyboard Shortcuts

| Key | Mode | Description |
|-----|------|-------------|
| `:` | Normal | Enter command input mode |
| `Esc` | Command | Exit command input mode |
| `h` / `j` / `k` / `l` | Normal | Vim-style directional navigation |
| `Ctrl+C` | Any | Exit the TUI |

### Built-in Tools

| Tool | Parameters | Description |
|------|------------|-------------|
| `search_tools` | `query`, `max_results` | Search and inject tools on demand; supports `select:<name>` direct activation, keyword fuzzy search, and `+term` required-match syntax |
| `read_file` | `path`, `offset`, `limit` | Read file content; binary files are detected and rejected; PDF dispatched by extension (page-based); CSV/TSV emitted as JSON 2D array `[[header...], [row1...], ...]` (BOM stripped, header always included, rows normalized to header width) |
| `read_image` | `path` | Read a local image file (JPEG/PNG/GIF/WebP, max 10 MB) and return it as a base64 JPEG data URL |
| `write_file` | `path`, `content` | Write or create a file (atomic write) |
| `list_files` | `path`, `recursive` | List directory contents |
| `glob_files` | `pattern` | Glob pattern matching (e.g., `**/*.go`) |
| `search_files` | `pattern`, `file_pattern` | Regex search across file contents |
| `ask_user` | `questions` | Interactive prompt — free-text / single-select / multi-select via `promptui`; gated by `cli-*` session prefix only (v0.20.0). Other prefixes (`http-` / `dc-` / `temp-` / `temp-sub-`) return guidance text instructing the LLM to relay the question via reply text instead of blocking on stdin |
| `patch_file` | `path`, `old_string`, `new_string` | First-match string replace (safer than full rewrite) |
| `search_conversation_history` | `keyword`, `time_range` | Query the current session's history records from ToriiDB |
| `read_error_memory` | `hash` | Retrieve full error details for a failed tool call by hash |
| `remember_error` | `tool_name`, `keywords`, `symptom`, `action` | Persist tool error decisions to the error knowledge base |
| `search_error_memory` | `keyword` | Retrieve error knowledge base entries |
| `fetch_yahoo_finance` | `symbol`, `interval`, `range` | Fetch Yahoo Finance stock quotes and OHLCV candlesticks; concurrent query1/query2 fetch, returns fastest |
| `fetch_youtube_transcript` | `url` | YouTube video transcript with timestamps |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google News RSS feed with deduplication |
| `send_http_request` | `method`, `url`, `headers`, `body` | Generic HTTP request |
| `search_web` | `query`, `time_range` | DuckDuckGo lite-endpoint web search; `time_range` accepts `1d` / `7d` / `1m` / `1y` |
| `fetch_page` | `url` | JS-rendered page content as Markdown (headless Chrome) |
| `save_page_to_file` | `href`, `save_to` | JS-rendered page saved to a local file |
| `run_command` | `argv` | Execute whitelisted commands inside the OS sandbox; `argv: string[]` only — no shell-string parsing, multi-word args carried verbatim. Shell features (pipe / redirect / glob / `$VAR`) require explicit `["sh","-c","..."]` |
| `add_task` | `at`, `script`, `channel_id` | Schedule a one-time task; result posted to Discord channel on completion |
| `list_tasks` | — | List all pending one-time tasks |
| `remove_task` | `index` | Cancel and remove a one-time task |
| `add_cron` | `cron_expr`, `script`, `channel_id` | Register a recurring cron task; result posted to Discord after each run |
| `list_crons` | — | List all registered cron tasks |
| `remove_cron` | `index` | Remove a cron task by index |
| `skill_git_commit` | `message` | Commit current changes in the skill repository |
| `skill_git_log` | `limit` | Show recent commit history for the skill repository |
| `skill_git_rollback` | `commit` | Roll back the skill repository to the specified commit hash |
| `list_tools` | — | List all currently available tools including dynamic API extensions |
| `calculate` | `expression` | Evaluate math expressions (sqrt, abs, pow, ceil, floor, sin, cos, tan, log) |
| `invoke_external_agent` | `provider`, `task`, `readonly?` | Delegate the entire task to a named external CLI agent (`copilot` / `claude` / `codex` / `gemini`); `readonly` defaults to `true` |
| `cross_review_with_external_agents` | `input`, `result` | Parallel cross-validation: dispatch to all declared external agents and merge feedback; falls back to `review_result` when none are declared |
| `review_result` | `input`, `result` | Internal completeness review using the highest-priority available model (claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet) |
| `invoke_subagent` | `task`, `name?`, `session_id?`, `model?`, `system_prompt?`, `exclude_tools?` | In-process sub-agent delegation. `name` (v0.20.0) resolves a `cli-*`/`http-*` session by `bot.md` frontmatter `name` (takes precedence over `session_id`; emits error on miss). Always-excluded set: `invoke_subagent`, `invoke_external_agent`, `cross_review_with_external_agents`, `review_result`, `ask_user` (subagents return a single final text and cannot pause for interactive input) |

## Slash Command Routing

When user input begins with one of the prefixes below, the engine bypasses planner agent selection (and skill matching) and forwards the rest of the message directly to the named external CLI agent. Each agent is a one-shot `subprocess` invocation — no ACP, no JSON-RPC. Both the CLI / TUI / Discord and REST `/v1/send` entries respect these prefixes.

| Prefix | Agent | Mode |
|---|---|---|
| `/claude <task>` | Claude Code CLI | read-only (`--disallowedTools=Edit,Write,NotebookEdit`) |
| `/claude-allow <task>` | Claude Code CLI | write (`--permission-mode acceptEdits`) |
| `/codex <task>` | OpenAI Codex CLI | sandboxed read-only |
| `/codex-allow <task>` | OpenAI Codex CLI | bypass approvals + sandbox |
| `/gh <task>` · `/copilot <task>` | GitHub Copilot CLI | reasoning-only (no tool execution; no `-allow` variant) |
| `/gemini <task>` | Gemini CLI | `--approval-mode plan` (no mutating tools) |
| `/gemini-allow <task>` | Gemini CLI | `--yolo` (auto-approve all tools) |

The corresponding agent must be enabled via the `EXTERNAL_*` environment variable and have its CLI installed and authenticated locally.

## REST API

Start the unified app to expose the REST API on `PORT` (default: `17989`):

```bash
agen
# or: make app
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/send` | Execute agent and return response (SSE or JSON) |
| `GET` | `/v1/tools` | List all registered tools |
| `POST` | `/v1/tool/:name` | Invoke a single tool directly |
| `GET` | `/v1/key` | Retrieve a stored credential from the OS Keychain |
| `POST` | `/v1/key` | Save a credential to the OS Keychain |
| `GET` | `/v1/session/:session_id/status` | Read per-session online/idle state from `status.json` (404 if session directory missing) |
| `GET` | `/v1/session/:session_id/log` | SSE-stream `action.log` — initial backlog of the trailing 100 lines, then 1 s polling with last-line dedup; `: ping` heartbeat after 15 quiet ticks; closes when the client disconnects |

### POST /v1/send

Run the full agent execution loop. Set `"sse": true` to receive token chunks as a Server-Sent Events stream.

**Request:**
```json
{ "content": "summarize today's news", "sse": false }
```

Use the optional `model` field to bypass automatic agent selection and route directly to a specific model (key format: `provider@model-name`):

```json
{ "content": "summarize today's news", "sse": false, "model": "claude@claude-opus-4-6" }
```

Use `exclude_tools` to suppress specific tools for this request only:

```json
{ "content": "summarize today's news", "sse": false, "exclude_tools": ["run_command", "write_file"] }
```

**Session persistence (v0.20.0):**

| `session_id` | `persist` | Resulting prefix | Lifetime |
|---|---|---|---|
| set | (ignored) | uses caller-supplied id | caller-managed |
| empty | `false` (default) | `temp-<uuid>` | reaped after 1 h idle |
| empty | `true` | `http-<uuid>` | **permanent**; caller must persist `session_id` from the response to resume |

```json
{ "content": "start an ongoing research thread", "sse": false, "persist": true }
```

**Response (non-SSE):**
```json
{ "text": "..." }
```

**Response (SSE):** `Content-Type: text/event-stream` — each `data:` line is a token chunk; the stream closes when the agent finishes.

### GET /v1/tools

Returns all registered tools (built-in, API extensions, and script tools).

### POST /v1/tool/:name

Invoke a single tool by name. The request body is passed directly as the tool arguments.

**Request:**
```json
{ "query": "Bitcoin price", "time_range": "1d" }
```

### GET /v1/key · POST /v1/key

Read or write a credential entry in the OS Keychain. Script tools should use these endpoints rather than accessing the keychain directly.

### GET /v1/session/:session_id/status

Returns per-session liveness from `<sessions_dir>/<session_id>/status.json`:

```json
{
  "state": "online",
  "active": [{"id": "...", "input": "...", "started_at": "2026-04-26 ..."}],
  "ended_at": "",
  "limit": 3,
  "usage": 33.33
}
```

`state` is derived from `len(active) > 0` (`online` | `idle`); `ended_at` records the last time `active` drained; `limit` is `MAX_SESSION_TASKS`; `usage` is `len(active)/limit` as a percentage. Returns `404` when the session directory does not exist.

### GET /v1/session/:session_id/log

Server-Sent Events stream of the per-session `action.log`. On connect, the handler emits the trailing 100 lines as backlog (oldest first); thereafter it polls once per second, deduplicates by the last emitted line content, and pushes only newly appended lines. After 15 consecutive quiet ticks (no new lines) it sends a `: ping\n\n` SSE comment to keep intermediaries from idling out. The stream closes when the client disconnects.

```bash
curl -N "http://localhost:${PORT:-17989}/v1/session/<sid>/log"
```

Each event is a single `data: <line>\n\n` frame; lines preserve the action.log format `[YYYY-MM-DD HH:MM:SS.mmm][kind] body`.

### Calling the API from script tools

Script tools running inside scheduled tasks can call the API via `localhost`:

```python
import json, urllib.request, os

BASE = f"http://localhost:{os.environ.get('PORT', '17989')}"

def call_tool(name, args):
    payload = json.dumps(args).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/tool/{name}",
        data=payload, headers={"Content-Type": "application/json"}, method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("result", "")
```

## Sandbox Isolation

All commands executed via `run_command` and scheduler scripts run inside an OS-native sandbox:

| Feature | Linux (bwrap) | macOS (sandbox-exec) |
|---------|---------------|----------------------|
| Filesystem | Read-only root, writable `$HOME` | Deny-default, `file-read*` allowed, `file-write*` scoped to `$HOME` |
| Sensitive path denial | `--tmpfs` / `--ro-bind /dev/null` over sensitive paths | Seatbelt `deny file-read*` / `deny file-write*` |
| Namespace isolation | `--unshare-user/pid/ipc/uts/cgroup` (individually probed) | Not available |
| Session isolation | `--new-session` | Not available |
| Network | Allowed (`--share-net`) | Allowed (`allow network*`) |
| Orphan prevention | `--die-with-parent` | Not available |
| Path validation | `filepath.EvalSymlinks` → reject if outside `$HOME` | Same |
| Auto-install | Detected on startup; installs via package manager if missing | Built-in, no installation needed |

## Agent Interface

```go
type Agent interface {
    Name() string
    MaxInputTokens() int
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` handles a single LLM API call. `Execute` manages the complete skill execution loop with up to 128 tool call iterations, automatically triggering summarization at the limit. `MaxInputTokens` returns the model's maximum input token count, used for session-level token-budget trimming.

## Provider Registry

```go
func Default(provider string) string
func Get(provider, model string) ModelItem
func Models(provider string) map[string]ModelItem
func InputBytes(provider, model string) int
func OutputTokens(provider, model string) int
func SupportTemperature(provider, model string) bool
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
