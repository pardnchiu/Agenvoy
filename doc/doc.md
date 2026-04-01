# agenvoy - Documentation

> Back to [README](../README.md)

## Prerequisites

### System Requirements

- Go 1.20 or higher
- At least one AI provider credential (GitHub Copilot subscription, or any API key)
- Discord Bot Token (server mode only)

### Sandbox Dependencies

| Platform | Dependency | Notes |
|----------|-----------|-------|
| Linux | `bubblewrap` (`bwrap`) | Auto-detected on startup; if not installed, automatically installed via `apt-get` / `dnf` / `yum` / `pacman` / `apk` |
| macOS | `sandbox-exec` | Built into macOS, no installation required |

### Browser Dependencies (Optional)

- Chromium or Google Chrome — used by `fetch_page` and `download_page` tools in headless mode
- `go-rod` will auto-download Chromium on first use if not present on the system

### Go Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/bwmarrin/discordgo` | Discord Bot API |
| `github.com/gin-gonic/gin` | REST API server (HTTP routing) |
| `github.com/go-rod/rod` | Headless Chrome browser automation |
| `github.com/go-shiori/go-readability` | HTML content extraction and cleanup |
| `github.com/joho/godotenv` | `.env` environment variable loading |
| `github.com/manifoldco/promptui` | Interactive CLI selection menus |
| `github.com/pardnchiu/go-scheduler` | Cron expression parsing and scheduling |
| `github.com/rivo/tview` | Terminal UI framework |
| `github.com/gdamore/tcell/v2` | Terminal cell and event library |
| `github.com/fsnotify/fsnotify` | Filesystem event monitoring (TUI file watcher) |
| `golang.org/x/image` | WebP image decoding (vision input) |
| `golang.org/x/net` | HTML tokenizer and network utilities |
| `golang.org/x/term` | Terminal state and raw mode control |

## Installation

### Using go install

```bash
go install github.com/pardnchiu/agenvoy/cmd/cli@latest
```

### From Source (CLI)

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -o agenvoy ./cmd/cli
```

### From Source (Unified App: TUI + Discord + REST API)

```bash
go build -o agenvoy-app ./cmd/app
```

### From Source (Discord Bot only)

```bash
go build -o agenvoy-server ./cmd/server
```

## Configuration

### Adding a Provider

Run the interactive setup to select a provider and model from the embedded registry:

```bash
agenvoy add
```

Supported providers:

| Provider | Authentication | Default Model |
|----------|---------------|---------------|
| GitHub Copilot | OAuth Device Code Flow (auto-refresh) | `gpt-4.1` |
| OpenAI | API Key (keychain) | `gpt-5-mini` |
| Claude | API Key (keychain) | `claude-sonnet-4-5` |
| Gemini | API Key (keychain) | `gemini-2.5-pro` |
| NVIDIA | API Key (keychain) | `openai/gpt-oss-120b` |
| Compat | Optional API Key (keychain) | User-specified |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes (server mode) | Discord Bot Token |
| `DISCORD_GUILD_ID` | No | Restricts slash command registration to a specific guild |
| `PORT` | No | REST API server listen port (default: `17989`) |
| `MAX_HISTORY_MESSAGES` | No | Max history messages sent to agent (default: 16) |
| `MAX_TOOL_ITERATIONS` | No | Max tool call iterations per request (default: 16) |
| `MAX_SKILL_ITERATIONS` | No | Max tool call iterations within a skill execution (default: 128) |
| `MAX_EMPTY_RESPONSES` | No | Max consecutive empty responses before giving up (default: 8) |
| `EXTERNAL_COPILOT` | No | External agent endpoint for GitHub Copilot (used by `verify_with_external_agent` / `call_external_agent`) |
| `EXTERNAL_CLAUDE` | No | External agent endpoint for Claude (used by `verify_with_external_agent` / `call_external_agent`) |
| `EXTERNAL_CODEX` | No | External agent endpoint for Codex (used by `verify_with_external_agent` / `call_external_agent`) |

Create a `.env` file and fill in the values:

```bash
cp .env.example .env
```

> Files with `.example` in the name (e.g., `.env.example`) bypass the env prefix deny rule and are safe to read.

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
    },
    "status": {
      "type": "string",
      "description": "Filter by status",
      "required": false,
      "default": "active",
      "enum": ["active", "inactive", "all"]
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

The following API extensions are bundled and loaded automatically at startup:

| Extension | Category | Description |
|-----------|----------|-------------|
| `nominatim` | Geocoding | OpenStreetMap geocoding and reverse geocoding |
| `coingecko` | Finance | Cryptocurrency prices and market data |
| `yahoo-finance-1/2` | Finance | Stock quotes and historical data |
| `wikipedia` | Data | Wikipedia article search and content |
| `world-bank` | Data | World Bank development indicators |
| `usgs-earthquake` | Data | USGS earthquake feed |
| `themealdb` | Data | Recipe and meal database |
| `hackernews` | Data | Hacker News top stories and items |
| `rest-countries` | Data | Country information and metadata |
| `exchange-rate` | Finance | Currency exchange rates |
| `ip-api` | Network | IP geolocation lookup |
| `open-meteo` | Weather | Open-source weather forecast API |
| `youtube` | Media | YouTube video metadata (title, description, channel, duration) |

### Script Tool Extensions

Place a subdirectory containing `tool.json` + `script.js` or `script.py` in `~/.config/agenvoy/script_tools/` (or `<workdir>/.config/agenvoy/script_tools/`). The executor scans both paths on startup and registers each tool with the `script_` prefix.

#### Bundled Extension Installers

The repository ships ready-to-use script tool extensions with cross-platform install scripts:

```bash
# Install Threads API tools (publish text/image/carousel, quota check, token refresh)
bash install_threads.sh

# Install yt-dlp tools (video info, download with sanitized filenames)
bash install_youtube.sh
```

Both scripts detect the OS, verify Python and required packages, and copy the tools to `~/.config/agenvoy/script_tools/`. After installation, the tools are auto-registered as `script_`-prefixed tools on the next agent startup.

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

`tool.json` format:

```json
{
  "name": "my_tool",
  "description": "What the agent sees when selecting this tool",
  "parameters": {
    "type": "object",
    "properties": {
      "input": {
        "type": "string",
        "description": "Input value"
      }
    },
    "required": ["input"]
  }
}
```

Script I/O contract — the executor pipes the tool parameters as JSON to stdin and reads the result from stdout:

```python
#!/usr/bin/env python3
import json, sys

params = json.loads(sys.stdin.read() or "{}")
result = {"output": params.get("input", "").upper()}
print(json.dumps(result))
```

```js
const chunks = [];
process.stdin.on("data", d => chunks.push(d));
process.stdin.on("end", () => {
  const params = JSON.parse(Buffer.concat(chunks).toString() || "{}");
  console.log(JSON.stringify({ output: (params.input || "").toUpperCase() }));
});
```

Use the `script-tool-creator` skill to scaffold new tools automatically:

```bash
agenvoy run-allow "create a script tool that fetches weather for a city"
```

### Skill Extensions

Skill extensions are Markdown files with a YAML frontmatter header. On startup, SyncSkills fetches any skill directories from `extensions/skills` in the GitHub repository that are not yet present locally, storing them in `~/.config/agenvoy/skills/`. The agent then scans all 9 standard paths to build the available skill list.

Skill file format (`SKILL.md`):

```markdown
---
name: my-skill
description: One-line summary shown to the agent for skill selection
---

# My Skill

Instructions the agent follows when this skill is selected...
```

Scan paths (in priority order):

| Priority | Path |
|----------|------|
| 1 | `~/.config/agenvoy/skills/` (synced from GitHub + user-defined) |
| 2–9 | XDG config dirs, home dir, and project-local paths |

## REST API

Start the unified app to expose the REST API on `PORT` (default: `17989`):

```bash
./agenvoy-app
# or: go run ./cmd/app
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/send` | Execute agent and return response (SSE or JSON) |
| `GET` | `/v1/tools` | List all registered tools |
| `POST` | `/v1/tool/:name` | Invoke a single tool directly |
| `GET` | `/v1/key` | Retrieve a stored credential from the OS Keychain |
| `POST` | `/v1/key` | Save a credential to the OS Keychain |

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

Use `exclude_tools` to suppress specific tools for this request only (does not affect other sessions):

```json
{ "content": "summarize today's news", "sse": false, "exclude_tools": ["run_command", "write_file"] }
```

**Response (non-SSE):**
```json
{ "text": "..." }
```

**Response (SSE):** `Content-Type: text/event-stream` — each `data:` line is a token chunk; the stream closes when the agent finishes.

### GET /v1/tools

Returns all registered tools (built-in, API extensions, and script tools).

**Response:**
```json
{
  "tools": [
    { "name": "search_web", "description": "...", "parameters": { ... } }
  ]
}
```

### POST /v1/tool/:name

Invoke a single tool by name. The request body is passed directly as the tool arguments.

**Request:**
```json
{ "query": "Bitcoin price", "time_range": "1d" }
```

**Response:**
```json
{ "result": "..." }
```

### GET /v1/key · POST /v1/key

Read or write a credential entry in the OS Keychain. Script tools should use these endpoints instead of accessing the keychain directly.

**POST request:**
```json
{ "service": "my-service", "key": "secret-value" }
```

**GET request:** `?service=my-service`

**GET response:**
```json
{ "key": "secret-value" }
```

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

def send(prompt):
    payload = json.dumps({"content": prompt, "sse": False}).encode()
    req = urllib.request.Request(
        f"{BASE}/v1/send",
        data=payload, headers={"Content-Type": "application/json"}, method="POST"
    )
    with urllib.request.urlopen(req) as resp:
        return json.load(resp).get("text", "")
```

---

## Usage

### Using Make

From the project root (requires source clone):

| Target | Command | Description |
|--------|---------|-------------|
| `make app` | `go run ./cmd/app/main.go` | Start unified app (TUI + Discord + REST API) |
| `make discord` | `go run ./cmd/server/main.go` | Start Discord bot server (legacy) |
| `make add` | `go run ./cmd/cli/ add` | Interactively add a provider/model |
| `make remove` | `go run ./cmd/cli/ remove` | Remove a configured provider |
| `make planner` | `go run ./cmd/cli/ planner` | Set the planner model |
| `make list` | `go run ./cmd/cli/ list` | List configured models |
| `make skill-list` | `go run ./cmd/cli/ list skill` | List available skills |
| `make cli <input...>` | `go run ./cmd/cli/ run <input>` | Run agent with tool confirmation |
| `make run <input...>` | `go run ./cmd/cli/ run-allow <input>` | Run agent with all tools auto-approved |

### Basic

List all configured models:

```bash
agenvoy list
```

List all available skills:

```bash
agenvoy list skills
```

Run in interactive mode (confirms each tool call before execution):

```bash
agenvoy run "analyze the architecture of this project"
```

### Advanced

Auto-approve mode (skip all confirmation prompts):

```bash
agenvoy run-allow "generate and write the README documentation"
```

Attach an image input:

```bash
agenvoy run --image ./screenshot.png "what does this image describe?"
```

Attach a file input:

```bash
agenvoy run --file ./report.pdf "summarize the key points of this report"
```

Remove a provider:

```bash
agenvoy remove
```

## CLI Reference

### Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `add` | `agenvoy add` | Interactively register an AI provider |
| `remove` | `agenvoy remove` | Remove a configured provider |
| `planner` | `agenvoy planner` | Set the planner (router) model |
| `reasoning` | `agenvoy reasoning` | Configure reasoning level for a provider |
| `list` | `agenvoy list [skills]` | List configured models or available skills |
| `run` | `agenvoy run <input...> [flags]` | Execute agentic workflow with interactive confirmation |
| `run-allow` | `agenvoy run-allow <input...> [flags]` | Execute with all tool calls auto-approved |

### Flags (run / run-allow)

| Flag | Description |
|------|-------------|
| `--image <path>` | Attach an image as input |
| `--file <path>` | Attach a file as input |

### Built-in Tools

| Tool | Parameters | Description |
|------|------------|-------------|
| `search_tools` | `query`, `max_results` | Search and inject tools on demand; supports `select:<name>` direct activation, keyword fuzzy search, and `+term` required-match syntax |
| `read_file` | `path`, `pages` | Read file content; binary files are detected and rejected; PDF files support `pages` range (e.g. `"1-5"`) |
| `write_file` | `path`, `content` | Write or create a file (atomic write) |
| `list_files` | `path`, `recursive` | List directory contents |
| `glob_files` | `pattern` | Glob pattern matching (e.g., `**/*.go`) |
| `search_content` | `pattern`, `file_pattern` | Regex search across file contents |
| `patch_edit` | `path`, `old_string`, `new_string` | First-match string replace (safer than full rewrite) |
| `search_history` | `keyword`, `time_range` | Query current session history records |
| `get_tool_error` | `hash` | Retrieve full error details for a failed tool call by hash |
| `remember_error` | `tool_name`, `keywords`, `symptom`, `action` | Persist tool error decisions to error knowledge base |
| `search_errors` | `keyword` | Retrieve error knowledge base entries |
| `analyze_youtube` | `url` | YouTube video metadata (title, description, channel, duration, view count) |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google News RSS feed with deduplication |
| `send_http_request` | `method`, `url`, `headers`, `body` | Generic HTTP request |
| `search_web` | `query`, `time_range` | Concurrent web search (Google + DuckDuckGo) |
| `fetch_page` | `url` | JS-rendered page content as Markdown (headless Chrome) |
| `download_page` | `href`, `save_to` | JS-rendered page saved to a local file |
| `run_command` | `command` | Execute whitelisted shell commands in sandbox (300s timeout) |
| `add_task` | `at`, `script`, `channel_id` | Schedule a one-time task; result is posted to the Discord channel on completion |
| `list_tasks` | — | List all pending one-time tasks |
| `remove_task` | `index` | Cancel and remove a one-time task (list first if multiple) |
| `add_cron` | `cron_expr`, `script`, `channel_id` | Register a recurring cron task; result is posted to the Discord channel after each run |
| `list_crons` | — | List all registered cron tasks |
| `remove_cron` | `index` | Remove a cron task by index (list first if multiple) |
| `skill_git_commit` | `message` | Commit current changes in the skill repository with the given message |
| `skill_git_log` | `limit` | Show recent commit history for the skill repository |
| `skill_git_rollback` | `commit` | Roll back the skill repository to the specified commit hash |
| `list_tools` | — | List all currently available tools including dynamic API extensions |
| `calculate` | `expression` | Evaluate math expressions (sqrt, abs, pow, ceil, floor, sin, cos, tan, log) |
| `call_external_agent` | `agent`, `input` | Delegate the entire task to a named external agent (`copilot` / `claude` / `codex`) |
| `verify_with_external_agent` | `input`, `result` | Parallel cross-validation: dispatch current result to all declared external agents and merge feedback; falls back to `review_result` when no agents are declared |
| `review_result` | `input`, `result` | Internal completeness review using the highest-priority available model (claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet); context is trimmed to draft + feedback after review |

### Sandbox Isolation

All commands executed via `run_command` and scheduler scripts run inside an OS-native sandbox:

| Feature | Linux (bwrap) | macOS (sandbox-exec) |
|---------|---------------|----------------------|
| Filesystem | Read-only root, writable `$HOME` | Deny-default, `file-read*` allowed, `file-write*` scoped to `$HOME` |
| Sensitive path denial | `--tmpfs` over sensitive dirs, `--ro-bind /dev/null` over sensitive files | Seatbelt `deny file-read*` / `deny file-write*` rules |
| Namespace isolation | `--unshare-user/pid/ipc/uts/cgroup` (individually probed for availability) | Not available |
| Session isolation | `--new-session` | Not available |
| Network | Allowed (`--share-net`) | Allowed (`allow network*`) |
| Orphan prevention | `--die-with-parent` | Not available |
| Path validation | `filepath.EvalSymlinks` → reject if outside `$HOME` | Same |
| Auto-install | Detected on startup; installs automatically via package manager if missing | Built-in, no installation needed |

### Token Usage Tracking

Every LLM API call returns input/output token counts. These are accumulated across all iterations within a single execution session (including tool-call loops and final summarization). The total is displayed on completion:

- **CLI**: `(elapsed) [model | in:N out:N]`
- **Discord**: footer line `-# model | in:N out:N`

Supported provider formats are handled transparently: Claude (`input_tokens`/`output_tokens`), OpenAI-compatible (`prompt_tokens`/`completion_tokens`), and Gemini (`promptTokenCount`/`candidatesTokenCount`) are all normalized to a unified `Usage` struct via custom `UnmarshalJSON`.

### Tool Error Tracking

When any tool call fails, the error is persisted to `tool_errors/{hash}.json` within the session directory and the agent receives `no data: {hash}`. The agent can call `get_tool_error` with the 8-character hex hash to retrieve the full error context (tool name, arguments, error message). Errors are also sent immediately via `EventExecError`: written to stderr in CLI mode, appended as a footer in Discord replies.

### Agent Interface

```go
type Agent interface {
    Name() string
    MaxInputTokens() int
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` handles a single LLM API call. `Execute` manages the complete skill execution loop with up to 128 tool call iterations, automatically triggering summarization at the limit. `MaxInputTokens` returns the model's maximum input token count, used for session-level token-budget trimming.

### Provider Registry

```go
// Get the default model name for a provider
func Default(provider string) string

// Get context limits and description for a specific model
func Get(provider, model string) ModelItem

// List all available models for a provider
func Models(provider string) map[string]ModelItem

// Calculate max input bytes (tokens × 4 for UTF-8)
func InputBytes(provider, model string) int

// Get max output token count
func OutputTokens(provider, model string) int

// Whether the model supports the temperature parameter
func SupportTemperature(provider, model string) bool
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
