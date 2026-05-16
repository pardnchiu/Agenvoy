# Getting Started

> [中文](https://github.com/agenvoy/Agenvoy/wiki/新手入門)

## Prerequisites

- Go 1.25.1 or higher
- Linux (sandbox via bubblewrap; auto-installs `bwrap` via apt/dnf/yum/pacman/apk if missing) or macOS (`sandbox-exec`)
- At least one LLM provider account (Copilot subscription, or an API key for OpenAI / Claude / Gemini / Nvidia)
- Optional: `pdftotext` (poppler-utils) for `read_file` to parse PDFs
- Optional: `OPENAI_API_KEY` to enable semantic search (`text-embedding-3-small`)

## Install

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
make build
```

`make build` compiles, embeds the latest git tag as `projectVersion`, and installs the binary to `/usr/local/bin/agen`.

## Configure at least one provider

Agenvoy needs at least one LLM provider to operate:

```bash
agen model add
```

The interactive prompt walks through provider selection, model choice, and credential storage. Tokens land in the OS keychain (`security` on macOS, `secret-tool` on Linux, encrypted file fallback otherwise) under the fixed service name `agenvoy`.

The main config lives at `~/.config/agenvoy/config.json`.

## First run

```bash
# Create a named cli- session and switch the primary pointer to it
agen session new my-assistant

# Launch the full stack (TUI + Discord + Telegram + REST)
make app
```

Once the TUI is up, press **`i`** to open the Message input and submit with **Enter** (`Shift+Enter` inserts a newline on terminals that forward modifiers). Press **`c`** to open the Command (`$`) input. **`Tab`** toggles between Content and Logs in the main view; **`Ctrl+P`** opens the co-work dashboard (Sessions / Log / Pending three-panel).

For one-shot CLI usage:

```bash
make cli "summarize the latest changes in main.go"
make run "use playwright to open example.com and screenshot"
```

`make cli` confirms each non-read-only tool call; `make run` auto-approves everything.

## Next steps

- [Core Concepts](https://github.com/agenvoy/Agenvoy/wiki/Core-Concepts) — sessions, agent routing, the iteration loop, and three-pass tool dispatch
- [Providers](https://github.com/agenvoy/Agenvoy/wiki/Providers) — supported LLM backends and the planner model
- [MCP Integration](https://github.com/agenvoy/Agenvoy/wiki/MCP-Integration) — plug in external tool servers
- [CLI Reference](https://github.com/agenvoy/Agenvoy/wiki/CLI-Reference) — full command list
