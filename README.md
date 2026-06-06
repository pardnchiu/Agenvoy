<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>Make AI actually work for you — your personal AI assistant.</strong>
</p>

<p align="center">
  Generative power and natural-language automation are the computing power of the AI era.<br><strong>Agenvoy is your productivity infrastructure.</strong>
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

Computing power defined what you could build in the computer age. In the AI era, that role belongs to generative capability and natural-language automation — and like computing power, it needs infrastructure you own.

Agenvoy is that infrastructure. A single Go binary on your machine turns Claude, GPT, and Gemini into a unified, always-on productivity layer. Configure it once, talk to it from anywhere — Telegram, Discord, LINE (alpha), terminal TUI, or browser. Same memory, same tools, same skills across every channel.

## What it can do

**Teach the agent to build its own tools — no programming required.** Just describe what you need; the agent writes a script or wires up an API, sandboxes it, and loads it as a tool. Next time you ask, it just runs.

| Demo · Auto-generate tools | Demo · Skill-based scheduler |
| :-: | :-: |
| [![](https://i.ytimg.com/vi/WBCjLQ-nQFo/maxresdefault.jpg)](https://www.youtube.com/watch?v=WBCjLQ-nQFo) | [![](https://i.ytimg.com/vi/bO9AMrW3L9c/maxresdefault.jpg)](https://www.youtube.com/watch?v=bO9AMrW3L9c) |
| **Demo · Sub-agents co-work** | **Demo · Tool install from registry** |
| [![](https://i.ytimg.com/vi/wM3NU4ARz4w/maxresdefault.jpg)](https://www.youtube.com/watch?v=wM3NU4ARz4w) | [![](https://i.ytimg.com/vi/UrR5i7YAHRc/maxresdefault.jpg)](https://www.youtube.com/watch?v=UrR5i7YAHRc) |

Out of the box it also:

- **Chats anywhere** — Telegram inline buttons · Discord select menus / modals · LINE (alpha) · terminal TUI · browser canvas. One daemon, every surface.
- **Speaks replies (TTS), transcribes audio / video.**
- **Schedules itself** — say "every weekday 8am push a Hacker News top-stories digest to Telegram" and the agent builds the cron + push pipeline for you.
- **Picks the right model per task** — Claude for coding, Gemini for video, GPT for research, routed automatically.
- **Searches your files semantically** — KuraDB indexes your local docs / notes (file → embedding vector); the agent answers from your own knowledge base, not generic training data.
- **Remembers across sessions** — three-tier memory: recent context + vector similarity (ToriiDB) + full-text archive (SQLite FTS5). Every message is dual-written; nothing is ever lost.
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
| Native chat buttons / menus / modals | ✅ inline keyboard / select / modal | ⚠️ reactions / text-based | ⚠️ text-based options |
| Agent builds & saves its own tools | ✅ FaaS-sandboxed scripts + APIs | ❌ | ⚠️ skill-only |
| First-contact verification on Telegram/Discord | ✅ 6-digit OTP | ⚠️ pairing code (manual approve) | ⚠️ pairing code (`gateway/pairing.py`) |
| Cross-session push (any session → chat) | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool (scope differs) |
| Native document RAG (file → embedding) | ✅ KuraDB in-process (semantic + keyword) | ❌ (conversation memory only) | ❌ (conversation memory only) |

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

***

### 2. AI Provider Support

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Claude | ✅ | ✅ | ✅ | ✅ only | ❌ | ❌ |
| OpenAI / GPT | ✅ | ✅ | ✅ | ❌ | ✅ only | ❌ |
| Gemini | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ only |
| Codex (OpenAI OAuth) | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| GitHub Copilot | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Nvidia NIM | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| OpenAI-compat | ✅ | ✅ Ollama/LM Studio | ✅ OpenRouter 200+ | ❌ | ❌ | ❌ |
| DeepSeek | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| xAI (Grok) | ✅ API key | ✅ | ✅ OAuth + API key | ❌ | ❌ | ❌ |
| Mistral | ❌ | ✅ | ⚠️ via OpenRouter (no dedicated) | ❌ | ❌ | ❌ |
| Dispatcher routing | ✅ dedicated dispatcher model | ❌ | ❌ | ❌ | ❌ | ❌ |

***

### 3. Runtime & Frontend

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| TUI | ✅ bubbletea | ✅ `openclaw tui` | ✅ React Ink | ✅ ink | ✅ | ✅ |
| CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| HTTP API / Web UI | ✅ gin | ✅ dashboard / webchat | ✅ Web Dashboard | ❌ | ❌ | ❌ |
| Daemon mode | ✅ native `--daemon` | ✅ systemd/launchd | ✅ gateway daemon | ❌ | ❌ | ❌ |
| Session Canvas (HTML+SSE) | ✅ `render_page` | ❌ | ❌ | ❌ | ❌ | ❌ |
| Named sessions | ✅ | ⚠️ workspaces / per-agent sessions | ✅ session picker | ❌ | ❌ | ❌ |

***

### 4. Chat Platform Integration

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Telegram | ✅ native daemon | ✅ native daemon | ✅ native daemon | ⚠️ Channels MCP (requires active session) | ❌ | ❌ |
| Discord | ✅ native daemon | ✅ native daemon | ✅ native daemon | ⚠️ Channels MCP (requires active session) | ❌ | ❌ |
| iMessage | ❌ | ✅ BlueBubbles | ✅ BlueBubbles | ⚠️ Channels MCP (macOS only) | ❌ | ❌ |
| LINE | ⚠️ alpha ([linebot branch](https://github.com/pardnchiu/Agenvoy/tree/linebot)) | ✅ | ✅ | ❌ | ❌ | ❌ |
| WhatsApp / Slack | ❌ | ✅ 24+ platforms | ✅ 24+ platforms | ❌ | ❌ | ❌ |
| Always-on receiving (no session needed) | ✅ daemon | ✅ | ✅ | ❌ | ❌ | ❌ |
| Cross-session send (any session → chat) | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ | ❌ | ❌ |
| First-contact verification | ✅ 6-digit OTP (crypto/rand) | ✅ pairing code (dmPolicy: pairing) | ✅ pairing code (`gateway/pairing.py`) | ❌ | ❌ | ❌ |
| Native platform UI (buttons / menus / modals) | ✅ inline keyboard / select menu / modal | ⚠️ reactions / text-based | ⚠️ text-based options | ❌ | ❌ | ❌ |

> **Platform layer**: Agenvoy's Telegram and Discord integrations are both built on [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot), independently maintained and open source. go-bot encapsulates the bot protocol details for both platforms — Agenvoy only implements business logic, while the platform API layer is entirely handled by go-bot.

> **Key difference**: Claude Code Channels requires an active session. OpenClaw and Hermes have daemons but their in-chat confirmations are largely text/reaction-based. Agenvoy uses native platform UI — Telegram inline keyboards and Discord select menus / modals. Additionally, Agenvoy's cross-session send tools let any session type (CLI, TUI, HTTP, scheduled script) push to a specific Telegram/Discord chat — a capability competitors expose only partially (e.g. Hermes' `send_message` within its own gateway scope).

***

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
| Format reference (lazy-load tool) | ✅ `format_chatbot` | ❌ | ❌ | ❌ |
| Scheduler output push | ✅ | ✅ | ✅ | ❌ |
| Cross-session push (from any session) | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ |
| Offline receiving (daemon) | ✅ | ✅ | ✅ | ❌ |

***

### 6. Discord Feature Comparison

| Feature | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| Send text reply | ✅ | ✅ | ✅ | ✅ |
| Send voice (TTS) | ✅ Gemini TTS → OGG/OPUS | ✅ | ✅ | ❌ |
| Send file attachments | ✅ batch 10/message | ✅ | ✅ | ❌ |
| Receive user attachments | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
| Tool confirm (interactive) | ✅ select menu button | ✅ `/model` picker | ⚠️ text options | ❌ |
| ask_user (modal) | ✅ select/multi-select/modal | ⚠️ limited | ⚠️ text options | ❌ |
| Format reference (lazy-load tool) | ✅ `format_chatbot` | ❌ | ❌ | ❌ |
| Guild mention guard | ✅ | ✅ | ✅ | ❌ |
| Discord Markdown aware | ✅ full spec as lazy-load tool | ⚠️ partial | ⚠️ partial | ❌ |
| Character limit aware | ✅ 1600 char hard limit in prompt | ❌ | ❌ | ❌ |
| Cross-session push (from any session) | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ |

***

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

***

### 8. Tool Ecosystem

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| MCP support | ✅ client | ✅ client | ✅ client + server | ✅ client | ❌ | ✅ client |
| Custom tools (script-tool-add) | ✅ AI-generated | ❌ | ✅ auto-creates skill | ❌ | ❌ | ❌ |
| API tool discovery (search-api → add) | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Tool registry (publish + install across machines) | ✅ pkg.agenvoy.com (Cloudflare Worker + R2 + D1, email verification + downgrade guard) | ⚠️ ClawHub (skills + plugins) | ⚠️ agentskills.io (skills only) | ❌ | ❌ | ❌ |
| Skill system | ✅ SKILL.md lazy-load | ✅ SKILL.md 5400+ community | ✅ SKILL.md agentskills.io | ✅ CLAUDE.md | ❌ | ❌ |
| Format reference as lazy-load tool | ✅ `format_chatbot` | ❌ | ❌ | ❌ | ❌ | ❌ |
| Document RAG (external knowledge base) | ✅ KuraDB (in-process vector + semantic/keyword) | ❌ (conversation memory only) | ❌ (conversation memory only) | ❌ | ❌ | ❌ |
| Media transcription STT | ✅ Gemini, 14 formats | ✅ Whisper/Gemini | ✅ faster-whisper (local) | ❌ | ❌ | ❌ |
| TTS voice output | ✅ Gemini TTS | ✅ ElevenLabs/Hume/MS | ✅ Edge TTS/ElevenLabs/OpenAI | ❌ | ❌ | ❌ |
| Computer use / browser | ✅ go-rod + Playwright MCP | ✅ Chrome CDP | ✅ browser CDP + computer-use (cua-driver) | ✅ beta | ❌ | ❌ |

> **Tool sandbox architecture**: Agenvoy's Python/JavaScript/API custom tool interfaces are built on the [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas) (Function as a Service) concept. Each AI-generated tool runs as an isolated function unit with its own lifecycle and security boundary. This is the only FaaS-level sandbox design for tool extension among all compared products.

***

### 9. Memory System

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Instruction file system | ✅ SKILL.md | ✅ SKILL.md | ✅ SKILL.md | ✅ CLAUDE.md | ❌ | ❌ |
| Conversation history search | ✅ Three-tier: context + ToriiDB vector + SQLite FTS5 | ✅ LanceDB vector | ✅ SQLite FTS5 | ❌ | ❌ | ❌ |
| External document RAG (native, in-process) | ✅ KuraDB (semantic + keyword, OpenAI embeddings) | ❌ (use MCP) | ❌ (use MCP) | ❌ | ❌ | ❌ |
| Error memory | ✅ ToriiDB | ❌ | ❌ | ❌ | ❌ | ❌ |
| Action log | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Long-term persistent memory | ✅ SQLite full-text archive (dual-write, never loses data) | ✅ Wiki-style MEMORY.md | ✅ MEMORY.md + USER.md | ⚠️ CLAUDE.md manual | ❌ | ❌ |
| Cross-session memory | ⚠️ session-isolated by default, extensible with external memory | ✅ built-in cross-session | ✅ built-in cross-session | ⚠️ session-isolated by default, extensible with external memory | ⚠️ session-isolated by default | ⚠️ session-isolated by default |

> **Three-tier conversation memory**: (1) **Context** — latest 16 messages loaded directly into LLM context + periodic summary; (2) **ToriiDB** — self-developed embedded vector database ([pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)) for semantic similarity search on recent conversations; (3) **SQLite FTS5** — full-text archive via [pardnchiu/go-sqlite](https://github.com/pardnchiu/go-sqlite), dual-written on every message, never loses data even after history compaction. `search_chat_history` routes by `mode`: `semantic` → ToriiDB, `keyword` → SQLite FTS5.

***

### 10. Dependencies & Deployment

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Direct external dependencies | **12** | large (pnpm monorepo) | 30–40 core + 60+ optional | 50+ | 40+ | 40+ |
| Self-maintained ecosystem packages | 6 (go-bot / go-pkg / go-scheduler / ToriiDB / go-faas / KuraDB) | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go (static binary) | Node.js | Python | Node.js | Node.js + Rust | Node.js |
| Deployment | **single binary** | npm install | pip + docker/VPS | npm install | npm install | npm install |

***

### Where Agenvoy Stands

| Dimension | Detail |
|-----------|--------|
| **Clear advantages** | Single Go binary, 12 dependencies, self-maintained ecosystem (pardnchiu universe), dispatcher model routing, Session Canvas, native platform UI (real buttons/modals), OTP verification, cross-session send to Telegram/Discord from any session, API tool auto-discovery, format reference as lazy-load tool, local-only scheduler (no cloud required) |
| **On par with competitors** | Telegram/Discord daemon, TTS/STT, scheduler output push, Skill system, MCP, browser automation, inbound attachment handling, provider coverage (compat layer covers any OpenAI-compatible endpoint) |
| **Where competitors lead** | Hermes context compression engine (token-budget compaction: head preservation + middle-turn summarization + iterative recompression, vs Agenvoy's reactive trim-only), OpenClaw 24+ platforms, Hermes MCP server mode, Hermes local STT, OpenClaw/Hermes built-in cross-session memory, Claude Code Computer Use beta, Claude Code cloud cron/task |
| **Codex CLI** | Fewest features — CLI + TUI + OpenAI OAuth only, no daemon, no chat platforms, no scheduler |

</details>

## Wiki

- [Getting Started](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Getting-Started.md)
- [Architecture](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Architecture.md)
- [Core Concepts](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Core-Concepts.md)
- [Providers](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Providers.md)
- [Tools](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Tools.md)
- [Memory System](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Memory-System.md)
- [Skill System](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Skill-System.md)
- [MCP Integration](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/MCP-Integration.md)
- [Security and Sandbox](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Security-and-Sandbox.md)
- [CLI Reference](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/CLI-Reference.md)
- [Configuration](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Configuration.md)

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Community Contributors

<a href="https://github.com/pardnchiu/Agenvoy/issues/3">
  <img src="https://github.com/Azetry.png" width="40" height="40" alt="Azetry" style="border-radius:50%" />
</a>
<a href="https://github.com/pardnchiu/agenvoy/issues/49">
  <img src="https://github.com/oceanasd.png" width="40" height="40" alt="oceanasd" style="border-radius:50%" />
</a>

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

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
