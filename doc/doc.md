# agenvoy - Documentation

> Back to [README](../README.md)

## Prerequisites

- Go 1.25.1 or higher
- At least one AI agent credential (choose one or more):
  - GitHub Copilot subscription (interactive Device Code login)
  - `OPENAI_API_KEY` (OpenAI)
  - `ANTHROPIC_API_KEY` (Claude)
  - `GEMINI_API_KEY` (Gemini)
  - `NVIDIA_API_KEY` (NVIDIA NIM)
  - Local Ollama or any OpenAI-compatible service (compat provider, no API key required)
- Chrome browser (the `fetch_page` tool uses go-rod; it downloads automatically on first use)
- For Discord bot mode: a Discord Bot Token and (optionally) a Guild ID

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

### From Source (Discord Bot Server)

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
go build -o agenvoy-server ./cmd/server
```

### As a Library

```bash
go get github.com/pardnchiu/agenvoy
```

## Configuration

### Add a Provider (Interactive)

Run the `add` command to interactively register a provider. Credentials are stored in the OS keychain — no manual env var setup required.

```bash
agenvoy add
```

The prompt lists all supported providers:

```
? Select provider to add:
  GitHub Copilot
  OpenAI
  Claude
  Gemini
  Nvidia
  Compat
```

- **GitHub Copilot**: opens Device Code browser login, then prompts for model name
- **API-key providers** (OpenAI / Claude / Gemini / NVIDIA): prompts for API key (masked input), stored in OS keychain
- **Compat**: prompts for provider name, endpoint URL (default: `http://localhost:11434`), optional API key, and model name

### Credential Lookup Order

For each API key, the keychain package checks in order:
1. OS keychain (macOS Keychain / Linux `secret-tool`)
2. Environment variable with the same key name
3. `~/.config/agenvoy/.secrets` (file fallback for other platforms)

### Agent Config File

Create an agent list at `~/.config/agenvoy/config.json` or `./.config/agenvoy/config.json`:

```json
{
  "default_model": "claude@claude-sonnet-4-5",
  "models": [
    {
      "name": "claude@claude-sonnet-4-5",
      "description": "High-quality tasks, document generation, code analysis"
    },
    {
      "name": "openai@gpt-5-mini",
      "description": "General queries, fast responses"
    },
    {
      "name": "compat[ollama]@qwen3:8b",
      "description": "Local tasks, offline use"
    }
  ]
}
```

### Skill Files

Skills are Markdown files discovered from 9 standard paths at startup:

| Priority | Path |
|----------|------|
| 1 | `./skills/` |
| 2 | `./.claude/skills/` |
| 3 | `~/.skills/` |
| 4 | `~/.claude/skills/` |
| 5 | `~/.config/agenvoy/skills/` |
| 6–9 | XDG / system-level paths |

### Discord Bot Environment Variables

Copy `.env.example` and fill in values:

```bash
cp .env.example .env
```

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes | Discord bot token |
| `DISCORD_GUILD_ID` | No | Guild ID for instant slash command registration (development). Leave empty for global registration (up to 1 hour propagation). |

## Usage

### CLI — Basic

```bash
agenvoy run What is the current price of TSMC stock?
```

### CLI — With Image Input

```bash
agenvoy run Describe this chart --image ./chart.png
agenvoy run-allow Compare these two screenshots --image /tmp/before.png --image /tmp/after.png
```

### CLI — With File Input

```bash
agenvoy run Analyze this log --file ./app.log
agenvoy run-allow Compare two configs --file ./config.a.yaml --file ./config.b.yaml
```

### CLI — Automatic Mode

```bash
agenvoy run-allow Generate a commit message for the current changes
```

### Discord Bot Server

```bash
# Copy and fill in environment variables
cp .env.example .env

# Run the server
./agenvoy-server
```

The bot responds to:
- **Direct messages**: any message triggers the agentic loop
- **Channel messages**: only when the bot is @mentioned

### Library — Embedding the Execution Engine

```go
package main

import (
    "context"
    "fmt"

    "github.com/pardnchiu/agenvoy/internal/agents/exec"
    "github.com/pardnchiu/agenvoy/internal/agents/provider/claude"
    "github.com/pardnchiu/agenvoy/internal/agents/provider/openai"
    atypes "github.com/pardnchiu/agenvoy/internal/agents/types"
    "github.com/pardnchiu/agenvoy/internal/skill"
)

func main() {
    ctx := context.Background()

    claudeAgent, err := claude.New("claude@claude-sonnet-4-5")
    if err != nil {
        panic(err)
    }
    oaiAgent, err := openai.New("openai@gpt-5-mini")
    if err != nil {
        panic(err)
    }

    registry := atypes.AgentRegistry{
        Registry: map[string]atypes.Agent{
            "claude@claude-sonnet-4-5": claudeAgent,
            "openai@gpt-5-mini":        oaiAgent,
        },
        Entries: []atypes.AgentEntry{
            {Name: "claude@claude-sonnet-4-5", Description: "High-quality tasks"},
            {Name: "openai@gpt-5-mini", Description: "General queries"},
        },
        Fallback: claudeAgent,
    }

    selectorBot, _ := openai.New("openai@gpt-5-mini")
    scanner := skill.NewScanner()
    events := make(chan atypes.Event, 16)

    go func() {
        defer close(events)
        if err := exec.Run(ctx, selectorBot, registry, scanner, "Check TSMC stock price", nil, nil, events, true); err != nil {
            fmt.Println("Error:", err)
        }
    }()

    for ev := range events {
        switch ev.Type {
        case atypes.EventText:
            fmt.Println(ev.Text)
        case atypes.EventDone:
            fmt.Println("Done")
        }
    }
}
```

## CLI Reference

### Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `add` | `agenvoy add` | Interactively register a provider and store credentials in the OS keychain |
| `remove` | `agenvoy remove` | Interactively remove a configured provider |
| `list` | `agenvoy list` | List configured models |
| `list skills` | `agenvoy list skills` | List all discovered Skills |
| `run` | `agenvoy run <input...> [--image <path>]... [--file <path>]...` | Execute a task (interactive mode, confirms before each tool call) |
| `run-allow` | `agenvoy run-allow <input...> [--image <path>]... [--file <path>]...` | Execute a task (automatic mode, skips all confirmations) |

### Supported Agent Providers

| Provider | Auth Method | Default Model | Environment Variable | Image Input |
|----------|-------------|---------------|----------------------|-------------|
| `copilot` | Device Code interactive login | `gpt-4.1` | — | ✓ |
| `openai` | API Key | `gpt-5-mini` | `OPENAI_API_KEY` | ✓ |
| `claude` | API Key | `claude-sonnet-4-5` | `ANTHROPIC_API_KEY` | ✓ |
| `gemini` | API Key | `gemini-2.5-pro` | `GEMINI_API_KEY` | ✓ |
| `nvidia` | API Key | `openai/gpt-oss-120b` | `NVIDIA_API_KEY` | ✗ |
| `compat` | Optional API Key | any | `COMPAT_{NAME}_API_KEY` | depends |

Model format: `{provider}@{model-name}`, e.g. `claude@claude-opus-4-6`.<br>
Compat format: `compat[{name}]@{model}`, e.g. `compat[ollama]@qwen3:8b`.

### Built-in Tools

| Tool | Parameters | Description |
|------|------------|-------------|
| `read_file` | `path` | Read file content at the specified path |
| `list_files` | `path`, `recursive` | List files and subdirectories |
| `glob_files` | `pattern` | Find files matching a glob pattern (e.g. `**/*.go`) |
| `write_file` | `path`, `content` | Write or create a file |
| `patch_edit` | `path`, `old`, `new` | Exact string replacement (safer than write_file) |
| `search_content` | `pattern`, `file_pattern` | Regex search across file contents |
| `search_history` | `keyword`, `time_range` | Search current session history; supports `1d`/`7d`/`1m`/`1y` filter |
| `run_command` | `command` | Execute an allowlisted shell command |
| `fetch_page` | `url` | Fetch a page after JS rendering via Chrome (supports SPA) |
| `download_page` | `href`, `save_to` | Download a page as readable Markdown to a local file |
| `search_web` | `query`, `range` | DuckDuckGo search returning title/URL/snippet |
| `fetch_yahoo_finance` | `symbol`, `range` | Real-time stock quotes and candlestick data |
| `fetch_google_rss` | `keyword`, `time` | Google News RSS search |
| `fetch_weather` | `city` | Current weather and forecast (omit city for current IP location) |
| `send_http_request` | `url`, `method`, `headers`, `body` | Generic HTTP request |
| `calculate` | `expression` | Precision math (supports `^`, `sqrt`, `abs`, etc.) |

### Allowed Shell Commands

The `run_command` tool is restricted to the following commands:
`git`, `go`, `node`, `npm`, `yarn`, `pnpm`, `python`, `python3`, `pip`, `pip3`, `ls`, `cat`, `head`, `tail`, `pwd`, `mkdir`, `touch`, `cp`, `mv`, `rm`, `grep`, `sed`, `awk`, `sort`, `uniq`, `diff`, `cut`, `tr`, `wc`, `find`, `jq`, `echo`, `which`, `date`, `docker`, `podman`

### Security Restrictions

Both file tools and `run_command` enforce path-level access control via `internal/tools/file/embed/denied.json`. The following are always blocked:

| Category | Blocked |
|----------|---------|
| SSH | `.ssh/` directory, `id_rsa`, `authorized_keys`, `known_hosts`, etc. |
| Shell history | `.bash_history`, `.zsh_history`, `.zhistory` |
| Shell config | `.zshrc`, `.bashrc`, `.bash_profile`, `.zprofile`, `.zshenv` |
| Cloud credentials | `.aws/`, `.gcloud/`, `.docker/`, `.gnupg/` |
| Private keys | `.pem`, `.key`, `.p12`, `.pfx`, `.cer`, `.crt`, `.der` |
| Secrets | `.env`, `.env.*`, `.netrc`, `.git-credentials` |

## API Reference

### Agent Interface

```go
type Agent interface {
    Send(ctx context.Context, messages []Message, toolDefs []tools.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` performs a single LLM API call. `Execute` manages the full skill execution loop including tool iteration, caching, and session writes.

### AgentRegistry

```go
type AgentRegistry struct {
    Registry map[string]Agent  // Agent instances indexed by name
    Entries  []AgentEntry      // Agent descriptions for the Selector Bot
    Fallback Agent             // Default agent when routing fails
}
```

### exec.Run

```go
func Run(
    ctx      context.Context,
    bot      Agent,           // Selector Bot (lightweight model)
    registry AgentRegistry,   // Available agent list
    scanner  *skill.Scanner,  // Skill scanner
    input    string,          // User input
    images   []string,        // Image paths (optional)
    files    []string,        // File paths (optional, content embedded in prompt)
    events   chan<- Event,    // Event output channel
    allowAll bool,            // true = skip all tool confirmations
) error
```

### Event Types

```go
const (
    EventSkillSelect  // Skill matching started
    EventSkillResult  // Skill matched (or "none")
    EventAgentSelect  // Agent routing started
    EventAgentResult  // Agent selected (or "fallback")
    EventText         // Agent text output
    EventToolCall     // A tool is about to be called
    EventToolConfirm  // Awaiting user confirmation (allowAll=false)
    EventToolSkipped  // User skipped the tool
    EventToolResult   // Tool execution result
    EventDone         // Current request completed
)
```

### skill.NewScanner

```go
func NewScanner() *Scanner
```

Creates and runs a concurrent skill scan across 9 standard paths. When duplicate skill names are found, the first one discovered takes precedence.

### keychain.Get / keychain.Set

```go
func Get(key string) string        // Read from OS keychain, fallback to env var
func Set(key, value string) error  // Write to OS keychain
```

### discord.New

```go
func New(
    plannerAgent  agentTypes.Agent,
    agentRegistry agentTypes.AgentRegistry,
    skillScanner  *skill.SkillScanner,
) (*discordTypes.DiscordBot, error)
```

Creates and connects a Discord bot session, registers slash commands, and returns the bot handle. Returns `nil, nil` when `DISCORD_TOKEN` is not set.

### File Attachment from Agent Reply

Agents running in Discord mode can send local files by embedding a marker in their text reply:

```
[SEND_FILE:/absolute/path/to/file.png]
```

Multiple files are supported. The markers are stripped from the visible message text before sending.

### APIDocumentData (Custom API Config Schema)

```go
type APIDocumentData struct {
    Name        string                       // Tool name (auto-prefixed with api_)
    Description string                       // Tool description (used by LLM for routing)
    Endpoint    APIDocumentEndpointData      // URL, Method, ContentType, Timeout
    Auth        *APIDocumentAuthData         // Authentication (bearer/apikey/basic)
    Parameters  map[string]APIParameterData  // Parameter definitions (with required, default)
    Response    APIDocumentResponseData      // Response format (json or text)
}
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
