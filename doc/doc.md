# agenvoy - Documentation

> Back to [README](../README.md)

## Prerequisites

- Go 1.20 or higher
- At least one AI provider credential (GitHub Copilot subscription, or any API key)
- Discord Bot Token (server mode only)

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

### From Source (Discord Bot)

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

### Environment Variables (Discord Bot Only)

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes | Discord Bot Token |
| `DISCORD_GUILD_ID` | No | Restricts slash command registration to a specific guild |

Create a `.env` file and fill in the values:

```bash
DISCORD_TOKEN=your_token_here
DISCORD_GUILD_ID=optional_guild_id
```

### Custom API Tools

Place JSON config files in `~/.config/agenvoy/apis/` or `./examples/apis/` to add custom API tools. Files follow the OpenAI Tool Schema format with request templating and response parsing support.

## Usage

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
| `read_file` | `path` | Read file content at the specified path |
| `write_file` | `path`, `content` | Write or create a file |
| `list_files` | `path`, `recursive` | List directory contents |
| `glob_files` | `pattern` | Glob pattern matching (e.g., `**/*.go`) |
| `search_content` | `pattern`, `file_pattern` | Regex search across file contents |
| `patch_edit` | `path`, `old`, `new` | String replace editing |
| `search_history` | `query` | Query session history records |
| `remember_error` | `key`, `decision` | Store tool error decisions |
| `search_errors` | `query` | Retrieve error knowledge base |
| `fetch_yahoo_finance` | `symbol`, `interval`, `range` | Stock market data |
| `fetch_google_rss` | `keyword`, `time`, `lang` | Google News RSS feed |
| `send_http_request` | `method`, `url`, `headers`, `body` | Generic HTTP request |
| `fetch_weather` | `city`, `days`, `hourly_interval` | Weather information |
| `search_web` | `query`, `time_range` | DuckDuckGo web search |
| `fetch_page` | `url` | JS-rendered page to Markdown (read-only) |
| `download_page` | `url`, `path` | JS-rendered page saved to file |
| `run_command` | `command` | Execute whitelisted shell commands |
| `calculate` | `expression` | Math expressions (sqrt, sin, cos, pow, etc.) |

### Agent Interface

```go
type Agent interface {
    Name() string
    Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}
```

`Send` handles a single LLM API call. `Execute` manages the complete skill execution loop with up to 128 tool call iterations, automatically triggering summarization at the limit.

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
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)
