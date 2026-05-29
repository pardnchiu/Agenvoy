<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>Make AI actually work for you — your personal AI assistant.</strong>
</p>

<p align="center">
  <strong>Say it. It builds the tool. No programming required.</strong> Claude / GPT / Gemini auto-routed; lives in Telegram, Discord, and your terminal.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/pardnchiu/agenvoy"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
  <a href="https://app.codecov.io/github/pardnchiu/agenvoy/tree/master"><img src="https://img.shields.io/codecov/c/github/pardnchiu/agenvoy/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
  <a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <strong>English</strong> · <a href="./doc/README.zh.md">繁體中文</a>
</p>

***

## What is Agenvoy

A **personal AI agent** that runs on your own machine. Configure it once, talk to it from anywhere — Telegram, Discord, terminal TUI, or browser. Same memory, same tools, same skills across every channel.

Built for people who want their own always-on assistant, not another SaaS subscription.

## What it can do

**Teach the agent to build its own tools — no programming required.** Just describe what you need; the agent writes a script or wires up an API, sandboxes it, and loads it as a tool. Next time you ask, it just runs.

| Demo · Auto-generate tools | Demo · Skill-based scheduler |
| --- | --- |
| [![](https://i.ytimg.com/vi/WBCjLQ-nQFo/maxresdefault.jpg)](https://www.youtube.com/watch?v=WBCjLQ-nQFo) | [![](https://i.ytimg.com/vi/bO9AMrW3L9c/maxresdefault.jpg)](https://www.youtube.com/watch?v=bO9AMrW3L9c) |
| **Demo · Sub-agents co-work** | **Demo · Tool install from registry** |
| [![](https://i.ytimg.com/vi/wM3NU4ARz4w/maxresdefault.jpg)](https://www.youtube.com/watch?v=wM3NU4ARz4w) | [![](https://i.ytimg.com/vi/UrR5i7YAHRc/maxresdefault.jpg)](https://www.youtube.com/watch?v=UrR5i7YAHRc) |

Out of the box it also:

- **Chats anywhere** — Telegram inline buttons · Discord select menus / modals · terminal TUI · browser canvas. One daemon, every surface.
- **Generates images, speaks replies (TTS), transcribes audio / video.**
- **Schedules itself** — say "every weekday 8am push a Hacker News top-stories digest to Telegram" and the agent builds the cron + push pipeline for you.
- **Picks the right model per task** — Claude for coding, Gemini for video, GPT for research, routed automatically.
- **Searches your files semantically** — KuraDB indexes your local docs / notes (file → embedding vector); the agent answers from your own knowledge base, not generic training data.
- **Remembers across sessions** — past conversations searchable by meaning, not just keywords.
- **Publishes and installs custom tools** — share AI-built tools across machines through the pkg.agenvoy.com registry; email-verified uploads with downgrade-proof versioning, one-popup install with dependency auto-resolve.

## One-line install

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

Single Go binary at `/usr/local/bin/agen`. macOS / Linux. No Node, no Python, no Docker.

> Running the daemon on a MacBook? `sudo pmset -c sleep 0` keeps the system awake while plugged in — prevents the daemon from being suspended on AC power.

## How it compares

Compared against the two closest peers — personal AI agent frameworks with daemon + chat-platform integration.

| You want… | **Agenvoy** | OpenClaw | Hermes |
|---|---|---|---|
| One-line install, single binary | ✅ Go | ❌ pnpm monorepo | ❌ pip + docker |
| Use Claude + GPT + Gemini in one chat | ✅ auto-routed by dispatcher | ✅ manual switch | ✅ manual switch |
| Native chat buttons / menus / modals | ✅ inline keyboard / select / modal | ⚠️ text-based options | ⚠️ text-based options |
| Agent builds & saves its own tools | ✅ FaaS-sandboxed scripts + APIs | ❌ | ⚠️ skill-only |
| First-contact verification on Telegram/Discord | ✅ 6-digit OTP | ⚠️ pairing code (manual approve) | ❌ allowlist only |
| Cross-session push (any session → chat) | ✅ `send_to_telegram_chat` / `send_to_discord_channel` | ❌ | ❌ |
| Image generation in chat | ✅ gpt-image-2 | ❌ | ❌ |
| Native document RAG (file → embedding) | ✅ KuraDB in-process (semantic + keyword) | ❌ (MCP only) | ❌ (MCP only) |

> Looking for the full feature-by-feature breakdown? See [**What makes it different**](#what-makes-it-different) below.

***

# For developers

## What makes it different

- **Dispatcher-based intelligent routing** — a dispatcher model routes every task to the best-fit worker (Claude for coding, Gemini for video, GPT for research), instead of forcing one model to do everything.
- **Agent that builds and persists its own tools** — when a tool is missing the agent writes a script or API into `extensions/` and loads it as a native tool on the next run; MCP servers are supported alongside.
- **One runtime across every channel** — Telegram, Discord, TUI, Web, and cron all attach to the same daemon; sessions, memory, and the tool set are shared, not rebuilt per surface.

<details>
<summary><strong>Agenvoy vs Mainstream Products: Full Detailed Comparison</strong></summary>

### 1. Overview

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| **Language** | Go | TypeScript | Python | TypeScript | Rust + TypeScript | TypeScript |
| **License** | Apache 2.0 | MIT | MIT | Proprietary | Apache 2.0 | Apache 2.0 |
| **Author** | Individual (pardnchiu) | Community | NousResearch | Anthropic | OpenAI | Google |
| **Primary use** | Multi-platform AI Agent framework | Multi-platform AI Agent | Multi-platform AI Agent | Terminal coding assistant | Terminal coding assistant | Terminal coding assistant |
| **Architecture** | Daemon + TUI + Chat | Daemon + TUI + Chat | Daemon + TUI + Chat | CLI session | CLI session | CLI session |

---

### 2. AI Provider Support

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Claude | ✅ | ✅ | ✅ | ✅ only | ❌ | ❌ |
| OpenAI / GPT | ✅ | ✅ | ✅ | ❌ | ✅ only | ❌ |
| Gemini | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ only |
| Codex (OpenAI OAuth) | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ |
| GitHub Copilot | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| Nvidia NIM | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| OpenAI-compat | ✅ | ✅ Ollama/LM Studio | ✅ OpenRouter 200+ | ❌ | ❌ | ❌ |
| DeepSeek | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| xAI (Grok) | ✅ API key | ✅ | ✅ OAuth + API key | ❌ | ❌ | ❌ |
| Mistral | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Dispatcher routing | ✅ dedicated dispatcher model | ❌ | ❌ | ❌ | ❌ | ❌ |

---

### 3. Runtime & Frontend

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| TUI | ✅ bubbletea | ✅ `openclaw tui` | ✅ React Ink | ✅ ink | ✅ | ✅ |
| CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| HTTP API / Web UI | ✅ gin | ❌ | ✅ Web Dashboard | ❌ | ❌ | ❌ |
| Daemon mode | ✅ native `--daemon` | ✅ systemd/launchd | ✅ gateway daemon | ❌ | ❌ | ❌ |
| Session Canvas (HTML+SSE) | ✅ `update_page` | ❌ | ❌ | ❌ | ❌ | ❌ |
| Named sessions | ✅ | ❌ | ✅ session picker | ❌ | ❌ | ❌ |

---

### 4. Chat Platform Integration

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Telegram | ✅ native daemon | ✅ native daemon | ✅ native daemon | ⚠️ Channels MCP (requires active session) | ❌ | ❌ |
| Discord | ✅ native daemon | ✅ native daemon | ✅ native daemon | ⚠️ Channels MCP (requires active session) | ❌ | ❌ |
| iMessage | ❌ | ✅ BlueBubbles | ✅ BlueBubbles | ⚠️ Channels MCP (macOS only) | ❌ | ❌ |
| WhatsApp / Slack / LINE | ❌ | ✅ 50+ platforms | ✅ 20+ platforms | ❌ | ❌ | ❌ |
| Always-on receiving (no session needed) | ✅ daemon | ✅ | ✅ | ❌ | ❌ | ❌ |
| Cross-session send (any session → chat) | ✅ `send_to_telegram_chat` / `send_to_discord_channel` | ❌ | ❌ | ❌ | ❌ | ❌ |
| First-contact verification | ✅ 6-digit OTP (crypto/rand) | ✅ pairing code (dmPolicy: pairing) | ❌ (allowlist only) | ❌ | ❌ | ❌ |
| Native platform UI (buttons / menus / modals) | ✅ inline keyboard / select menu / modal | ⚠️ text-based options | ⚠️ text-based options | ❌ | ❌ | ❌ |

> **Platform layer**: Agenvoy's Telegram and Discord integrations are both built on [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot), independently maintained and open source. go-bot encapsulates the bot protocol details for both platforms — Agenvoy only implements business logic, while the platform API layer is entirely handled by go-bot.

> **Key difference**: Claude Code Channels requires an active session. OpenClaw and Hermes have daemons but their in-chat confirmations are text-based. Agenvoy uses native platform UI — Telegram inline keyboards and Discord select menus / modals. Additionally, Agenvoy's cross-session send tools allow any session type (CLI, TUI, HTTP, scheduled script) to push messages to Telegram/Discord — no competitor exposes this capability.

---

### 5. Telegram Feature Comparison

| Feature | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| Send text reply | ✅ | ✅ | ✅ | ✅ |
| Send voice (TTS) | ✅ Gemini TTS → OGG | ✅ ElevenLabs/Hume | ✅ Edge TTS/ElevenLabs | ❌ |
| Send file attachments | ✅ `[SEND_FILE:]` | ✅ | ✅ | ❌ |
| Receive user attachments | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
| Voice-to-text (STT) | ✅ Gemini, 14 formats | ✅ Whisper/Gemini | ✅ faster-whisper (local) | ❌ |
| Tool confirm (interactive) | ✅ native inline keyboard | ⚠️ text approval prompt | ⚠️ text options | ❌ |
| ask_user (picker) | ✅ native button/modal | ⚠️ `/models` picker | ⚠️ text options, up to 4 | ❌ |
| Format reference (lazy-load tool) | ✅ `telegram_format` | ❌ | ❌ | ❌ |
| Scheduler output push | ✅ | ✅ | ✅ | ❌ |
| Cross-session push (from any session) | ✅ `send_to_telegram_chat` | ❌ | ❌ | ❌ |
| Offline receiving (daemon) | ✅ | ✅ | ✅ | ❌ |

---

### 6. Discord Feature Comparison

| Feature | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| Send text reply | ✅ | ✅ | ✅ | ✅ |
| Send voice (TTS) | ✅ Gemini TTS → OGG/OPUS | ✅ | ✅ | ❌ |
| Send file attachments | ✅ batch 10/message | ✅ | ✅ | ❌ |
| Receive user attachments | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
| Tool confirm (interactive) | ✅ select menu button | ✅ `/model` picker | ⚠️ text options | ❌ |
| ask_user (modal) | ✅ select/multi-select/modal | ⚠️ limited | ⚠️ text options | ❌ |
| Format reference (lazy-load tool) | ✅ `discord_format` | ❌ | ❌ | ❌ |
| Guild mention guard | ✅ | ✅ | ✅ | ❌ |
| Discord Markdown aware | ✅ full spec as lazy-load tool | ⚠️ partial | ⚠️ partial | ❌ |
| Character limit aware | ✅ 1600 char hard limit in prompt | ❌ | ❌ | ❌ |
| Cross-session push (from any session) | ✅ `send_to_discord_channel` | ❌ | ❌ | ❌ |

---

### 7. Scheduler

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Cron jobs | ✅ SKILL.md + cron | ✅ built-in | ✅ built-in | ✅ cloud-assisted cron/task | ❌ | ❌ |
| One-shot tasks | ✅ | ✅ `at` format | ✅ natural language | ✅ cloud-assisted | ❌ | ❌ |
| TUI CRUD | ✅ | ✅ `openclaw cron` | ✅ `cronjob` tool | ❌ | ❌ | ❌ |
| fsnotify hot-reload | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Push output to Telegram/Discord | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| AI tool management (add/list/remove) | ✅ | ❌ | ✅ `cronjob` tool | ❌ | ❌ | ❌ |
| Local execution (no cloud required) | ✅ | ✅ | ✅ | ❌ cloud-dependent | ❌ | ❌ |

> **Scheduler layer**: Agenvoy's scheduler is built on [pardnchiu/go-scheduler](https://github.com/pardnchiu/go-scheduler), a self-maintained ecosystem package providing cron expression parsing, one-shot tasks, fsnotify hot-reload, and full output routing back to chat platforms.

---

### 8. Tool Ecosystem

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| MCP support | ✅ client | ✅ client | ✅ client + server | ✅ client | ❌ | ✅ client |
| Custom tools (script-tool-add) | ✅ AI-generated | ❌ | ✅ auto-creates skill | ❌ | ❌ | ❌ |
| API tool discovery (search-api → add) | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Tool registry (publish + install across machines) | ✅ pkg.agenvoy.com (Cloudflare Worker + R2 + D1, email verification + downgrade guard) | ❌ | ⚠️ agentskills.io (skills only) | ❌ | ❌ | ❌ |
| Skill system | ✅ SKILL.md lazy-load | ✅ SKILL.md 5400+ community | ✅ SKILL.md agentskills.io | ✅ CLAUDE.md | ❌ | ❌ |
| Format reference as lazy-load tool | ✅ `telegram_format` / `discord_format` | ❌ | ❌ | ❌ | ❌ | ❌ |
| Image generation | ✅ DALL-E/Codex Image | ❌ | ❌ | ❌ | ❌ | ❌ |
| Document RAG (external knowledge base) | ✅ KuraDB (in-process vector + semantic/keyword) | ❌ (conversation memory only) | ❌ (conversation memory only) | ❌ | ❌ | ❌ |
| Media transcription STT | ✅ Gemini, 14 formats | ✅ Whisper/Gemini | ✅ faster-whisper (local) | ❌ | ❌ | ❌ |
| TTS voice output | ✅ Gemini TTS | ✅ ElevenLabs/Hume/MS | ✅ Edge TTS/ElevenLabs/OpenAI | ❌ | ❌ | ❌ |
| Computer use / browser | ✅ go-rod + Playwright MCP | ✅ Chrome CDP | ✅ Playwright (Chromium/Firefox) | ✅ beta | ❌ | ❌ |

> **Tool sandbox architecture**: Agenvoy's Python/JavaScript/API custom tool interfaces are built on the [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas) (Function as a Service) concept. Each AI-generated tool runs as an isolated function unit with its own lifecycle and security boundary. This is the only FaaS-level sandbox design for tool extension among all compared products.

---

### 9. Memory System

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Instruction file system | ✅ SKILL.md | ✅ SKILL.md | ✅ SKILL.md | ✅ CLAUDE.md | ❌ | ❌ |
| Conversation history search | ✅ ToriiDB vector search | ✅ SQLite vector | ✅ SQLite FTS5 | ❌ | ❌ | ❌ |
| External document RAG (native, in-process) | ✅ KuraDB (semantic + keyword, OpenAI embeddings) | ❌ (use MCP) | ❌ (use MCP) | ❌ | ❌ | ❌ |
| Error memory | ✅ ToriiDB | ❌ | ❌ | ❌ | ❌ | ❌ |
| Action log | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Long-term persistent memory | ⚠️ ToriiDB foundation in place | ✅ Wiki-style MEMORY.md | ✅ MEMORY.md + USER.md | ⚠️ CLAUDE.md manual | ❌ | ❌ |
| Cross-session memory | ⚠️ session-isolated by default, extensible with external memory | ✅ built-in cross-session | ✅ built-in cross-session | ⚠️ session-isolated by default, extensible with external memory | ⚠️ session-isolated by default | ⚠️ session-isolated by default |

> **ToriiDB** is a self-developed embedded vector database ([pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)) in the Agenvoy ecosystem. It requires no external service and runs in-process. Agenvoy uses ToriiDB as its memory infrastructure, currently powering semantic conversation history search and error memory, and serving as the foundation for future long-term cross-session memory expansion.

---

### 10. Dependencies & Deployment

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Direct external dependencies | **12** | large (pnpm monorepo) | 30–40 core + 60+ optional | 50+ | 40+ | 40+ |
| Self-maintained ecosystem packages | 6 (go-bot / go-pkg / go-scheduler / ToriiDB / go-faas / KuraDB) | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go (static binary) | Node.js | Python | Node.js | Node.js + Rust | Node.js |
| Deployment | **single binary** | npm install | pip + docker/VPS | npm install | npm install | npm install |

---

### Where Agenvoy Stands

| Dimension | Detail |
|-----------|--------|
| **Clear advantages** | Single Go binary, 12 dependencies, self-maintained ecosystem (pardnchiu universe), dispatcher model routing, Session Canvas, native platform UI (real buttons/modals), OTP verification, cross-session send to Telegram/Discord from any session, API tool auto-discovery, image generation, format reference as lazy-load tool, local-only scheduler (no cloud required) |
| **On par with competitors** | Telegram/Discord daemon, TTS/STT, scheduler output push, Skill system, MCP, browser automation, inbound attachment handling |
| **Where competitors lead** | OpenClaw 50+ platforms, Hermes MCP server mode, Hermes local STT, OpenClaw/Hermes built-in cross-session memory, Claude Code Computer Use beta, Claude Code cloud cron/task |
| **Codex CLI** | Fewest features — CLI + TUI + OpenAI OAuth only, no daemon, no chat platforms, no scheduler |

</details>

<details>
<summary><strong>CLI commands</strong></summary>

> Run as `agen <sub>`. `make <sub>` wrappers exist in the repo Makefile for development.

| Command | Description |
|---|---|
| `agen` | Attach interactive TUI; forks daemon (HTTP + Discord + Telegram + scheduler + summary cron) if not running. |
| `agen cli <input>` | One-shot agent run; every tool call asks for confirmation. |
| `agen run <input>` | One-shot agent run; auto-approves every tool call. |
| `agen stop` | Stop the running daemon (SIGTERM 5s grace → SIGKILL → clear `runtime.uid`). |
| `agen update` | Fetch latest release, rebuild, stop daemon — re-attach to load the new binary. |
| `agen model {add\|remove\|list\|dispatcher\|reasoning}` | Manage providers / worker models, pick dispatcher model, set reasoning level. |
| `agen mcp {list\|add\|remove}` | Manage MCP servers (stdio / HTTP) across global and per-session scope. |
| `agen session {new\|switch\|config} [name]` | Manage CLI sessions; bare `switch` / `config` opens an interactive picker. |

</details>

<details>
<summary><strong>TUI slash commands</strong></summary>

> Available inside `agen`'s TUI prompt. Type `/` to filter; popup commands transition cleanly back to the prompt.

| Command | Description |
|---|---|
| `/switch` | Switch active session via picker (current session pre-selected). |
| `/new [name]` | Create a new session; optional name pins it to the registry. Name is conflict-checked against existing sessions; abort on duplicate. |
| `/bot` | Edit the current session's bot via two sequential popups: name textfield (conflict-checked against other sessions; abort on conflict) → description textarea (`Ctrl+S` confirms, `Enter` newline, `Esc` cancels). |
| `/model [global\|session]` | Scope picker; `global` → `[add, remove]` (registry), `session` → pick a configured model. Inline arg skips the scope popup. |
| `/mcp [add\|remove]` | Action picker; `add` walks a chained popup form (name → transport → command/args/env or url/headers → scope → optional session pick), `remove` lists configured servers across global and session scopes. Restart the daemon to apply changes. Inline arg skips the action popup. |
| `/dispatcher-model` | Pick the dispatcher model from `cfg.Models` via popup. No inline arg. |
| `/summary-model` | Pick the model used for summary generation from `cfg.Models` (or `(use dispatcher)` to fall back). No inline arg. |
| `/reasoning [global\|session]` | Pick `low` / `medium` / `high` for the dispatcher (global) or the active session. Inline arg skips the scope popup. |
| `/discord [enable\|disable]` | Toggle Discord bot connection (token entry, verification, keychain write, daemon reload all happen in-TUI). Inline arg switches without the popup. |
| `/telegram [enable\|disable]` | Toggle Telegram bot connection (same in-TUI popup chain as `/discord`; first chat to message the bot must pass an in-chat verification code). Inline arg switches without the popup. |
| `/kuradb [enable\|disable]` | Toggle KuraDB RAG service. `enable` runs `install.sh` via `tea.ExecProcess` (sudo TTY handed back), prompts for `OPENAI_API_KEY` (stored in keychain), and writes `kuradb_enabled=true` — daemon picks up via fsnotify and spawns the child + endpoint file. `disable` removes `/usr/local/bin/kura` and clears the flag. Inline arg switches without the popup. |
| `/cron [add\|remove\|edit]` | Manage recurring schedules. `add` opens a multiline requirement textarea → dispatches `/scheduler-skill-creator <requirement>` (asks for missing when/what via `ask_user`). `remove` lists crons → confirm popup → `runtime.RemoveCron` + trashes the skill dir. `edit` lists crons → requirement textarea → agent picks `patch_cron` or rewrites the SKILL.md body. Inline arg skips the action popup. |
| `/task [add\|remove\|edit]` | Manage one-shot scheduled tasks (mirrors `/cron`; uses `add_task` / `patch_task` / `remove_task`). Picker shows `<YYYY-MM-DD HH:MM>  <skill>`. |
| `/sched-<name>` | Execute an existing scheduler skill body inline (manual trigger). Surfaced at the bottom of the `/` picker after regular skills; label rendered in warn-purple to mark it as an invocation. The dispatch wraps the body with an explicit "execute, do NOT activate scheduler-skill-creator" preamble. |
| `/mode [cli\|web]` | Switch between `cli` (TUI rendering) and `web` (browser page). Inline arg switches without the popup. |
| `/update` | Confirm popup → `agen stop && agen update` via `tea.ExecProcess` → quit TUI. |
| `/history` | Reload visible transcript — clear screen, reprint header, render the last 100 entries from the session's `action.log`. |
| `/log` | Open the raw `action.log` in `$PAGER` (fallback `less -Rf +G`, jumps to bottom). `\x1F` markers are expanded back to newlines for readability. |
| `/clear` | Clear the current window display only — like terminal `clear`; conversation memory is untouched. |
| `/exit`, `/quit` | Exit TUI (daemon keeps running; re-attach with `agen`). |

</details>

<details>
<summary><strong>Built-in tools</strong></summary>

> Tools auto-load on demand; stub names appear first, full schema activates on use. See [Tools wiki](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Tools.md) for parameters and routing.

| Tool | Description |
|---|---|
| **File** |  |
| `read_file` | Read a text, PDF, DOCX, PPTX, CSV/TSV, or image file. |
| `write_file` | Write content to a file, overwriting if it exists. |
| `patch_file` | Replace an exact string match inside a file. |
| `list_files` | List directory entries; `recursive=true` walks subtree files. |
| `glob_files` | Find files matching a glob pattern within a directory. |
| `search_files` | Search file contents by RE2 regex within a directory. |
| **Web** |  |
| `fetch_page` | Fetch a web page and return its content as Markdown. |
| `save_page_to_file` | Fetch a web page and save its content to a local file. |
| `search_web` | Search the web via DuckDuckGo Lite; returns top 10 results. |
| `fetch_google_rss` | Search Google News RSS and return article titles, summaries, links. |
| `fetch_yahoo_finance` | Query Yahoo Finance quotes and K-line (OHLCV). |
| `fetch_youtube_transcript` | Transcribe a YouTube video with timestamps. *(gemini needed)* |
| `transcribe_media` | Transcribe a local audio / video file (ogg, mp3, wav, m4a, flac, aac, mp4, mov, webm, mpeg, 3gp, …) up to 20 MiB. *(gemini needed)* |
| `send_http_request` | Send an HTTP request to a specified URL. |
| **Shell** |  |
| `run_command` | Run a binary with argv; returns combined stdout/stderr. |
| **Render** |  |
| `update_page` | Overwrite the rendered HTML page for the current session; tabs auto-reload. |
| `generate_image` | Generate an image via gpt-image-2 (size & quality picked by user). *(codex needed)* |
| **Channel** |  |
| `list_telegram_chat` | List authorized Telegram chats (id + name). *(telegram needed)* |
| `send_to_telegram_chat` | Send an HTML-formatted message to an authorized Telegram chat by chat_id. *(telegram needed)* |
| `telegram_format` | Return the Telegram HTML formatting reference (allowed tags, escape rules, file/voice markers). *(telegram needed)* |
| `list_discord_channel` | List authorized Discord channels (id + name). *(discord needed)* |
| `send_to_discord_channel` | Send a markdown-formatted message to an authorized Discord channel by channel_id. *(discord needed)* |
| `discord_format` | Return the Discord markdown formatting reference (allowed markdown, special tokens, file/voice markers). *(discord needed)* |
| **Calc** |  |
| `calculate` | Evaluate a mathematical expression and return the exact result. |
| **Discovery** |  |
| `list_tools` | List all currently available built-in and dynamically loaded tools. |
| `search_tools` | Search available tools by keyword and inject matches into the request. |
| `activate_skill` | Fetch a skill's reference material by exact name. |
| **Interactive** |  |
| `ask_user` | Ask the user one or more questions and return their answers. |
| `store_secret` | Prompt the user for a secret with masked input and persist to the system keychain. |
| **Memory** |  |
| `search_conversation_history` | Search the session's past messages by keyword and semantic similarity. |
| `search_error_memory` | Semantically search past tool-error records; hits refresh 3-month TTL. |
| `read_error_memory` | Fetch a prior tool-error record by hash. |
| `remember_error` | Persist a tool-error record for future retrieval. |
| **RAG (KuraDB)** |  |
| `rag_list_db` | List available KuraDB databases (e.g. notes, inbox, code). *(kuradb needed)* |
| `rag_search_keyword` | Keyword search a KuraDB database via gse tokenization. *(kuradb needed)* |
| `rag_search_semantic` | Semantic search a KuraDB database via OpenAI embeddings. *(kuradb needed)* |
| **Agent** |  |
| `invoke_subagent` | Run a subtask in an internal subagent session and return its final text. |
| `invoke_external_agent` | Invoke one external CLI agent (codex / copilot / claude / gemini) for a second opinion. |
| `cross_review_with_external_agents` | Cross-review a completed result across all available external agents in parallel. |
| `review_result` | Review a result against the original input and return issues and improvements. |
| **Scheduler** |  |
| `add_task` | Bind an existing scheduler skill to fire once at a specific time (`+5m` / `HH:MM` / `YYYY-MM-DD HH:MM` / RFC3339). |
| `add_cron` | Bind an existing scheduler skill to a recurring 5-field cron expression. |
| `patch_task` / `patch_cron` | Reschedule an existing task / cron by skill name (changes only the time, leaves the bound skill body untouched). |
| `remove_task` / `remove_cron` | Cancel a scheduled task / cron by skill name; the bound scheduler skill dir is moved to `.Trash/`. |
| **Skill Git** |  |
| `skill_git_commit` / `skill_git_log` / `skill_git_rollback` | Commit, list, or roll back the `~/.config/agenvoy/skills` git history. |

Dynamic tool families (auto-registered, not listed above): `mcp__<server>__<tool>` from configured MCP servers, `api_<name>` from `extensions/apis/*.json`, `script_<name>` from `extensions/scripts/<name>/`.

</details>

## Wiki

| English | 中文 |
|---|---|
| [Getting Started](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Getting-Started.md) | [新手入門](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Getting-Started.zh.md) |
| [Architecture](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Architecture.md) | [架構](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Architecture.zh.md) |
| [Core Concepts](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Core-Concepts.md) | [核心概念](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Core-Concepts.zh.md) |
| [Providers](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Providers.md) | [Provider 設定](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Providers.zh.md) |
| [Tools](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Tools.md) | [工具系統](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Tools.zh.md) |
| [Memory System](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Memory-System.md) | [記憶系統](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Memory-System.zh.md) |
| [Skill System](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Skill-System.md) | [Skill 系統](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Skill-System.zh.md) |
| [MCP Integration](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/MCP-Integration.md) | [MCP 整合](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/MCP-Integration.zh.md) |
| [Security and Sandbox](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Security-and-Sandbox.md) | [安全與沙箱](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Security-and-Sandbox.zh.md) |
| [CLI Reference](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/CLI-Reference.md) | [命令列參考](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/CLI-Reference.zh.md) |
| [Configuration](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Configuration.md) | [設定檔](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Configuration.zh.md) |

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Contributor

Just [open an issue](https://github.com/pardnchiu/agenvoy/issues/new) to share an idea.

<a href="https://github.com/pardnchiu/agenvoy/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=pardnchiu/agenvoy&cache_bust=2026-05-12" alt="Agenvoy contributors" />
</a>

## Star History

<a href="https://star-history.com/#pardnchiu/agenvoy&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&theme=dark&cache_bust=2026-05-12" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
    <img alt="Agenvoy star history" src="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
  </picture>
</a>

When the curve trends up — that's the signal we want to see. Hit ★ to push it along.

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
