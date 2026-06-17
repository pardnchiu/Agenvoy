# For Developers

## What makes it different

- **Dispatcher-based intelligent routing** — a dispatcher model routes every task to the best-fit worker (Claude for coding, Gemini for video, GPT for research), instead of forcing one model to do everything.
- **Agent that builds and persists its own tools** — when a tool is missing the agent writes a script or API into `extensions/` and loads it as a native tool on the next run; MCP servers are supported alongside.
- **One runtime across every channel** — Telegram, Discord, TUI, Web, and cron all attach to the same daemon; sessions, memory, and the tool set are shared, not rebuilt per surface.

## Agenvoy vs Mainstream Products: Full Comparison

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
| Native platform UI (buttons / menus / modals) | ✅ inline keyboard / select menu / modal | ⚠️ text-based options | ⚠️ text-based options | ❌ | ❌ | ❌ |

> **Platform layer**: Agenvoy's Telegram and Discord integrations are built on [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot), independently maintained and open source. go-bot encapsulates bot protocol details — Agenvoy only implements business logic.

> **Key difference**: Claude Code Channels requires an active session. OpenClaw and Hermes have daemons but in-chat confirmations are text-based. Agenvoy uses native platform UI — Telegram inline keyboards and Discord select menus / modals. Agenvoy's cross-session send lets any session type (CLI/TUI/HTTP/scheduled script) push to a specific Telegram/Discord chat — competitors expose this only partially.

***

### 5. Telegram Feature Comparison

| Feature | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| Text reply | ✅ | ✅ | ✅ | ✅ |
| Voice reply (TTS) | ✅ Gemini TTS → OGG | ✅ ElevenLabs/Hume | ✅ Edge TTS/ElevenLabs | ❌ |
| Send files | ✅ `[SEND_FILE:]` | ✅ | ✅ | ❌ |
| Receive attachments | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
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
| Text reply | ✅ | ✅ | ✅ | ✅ |
| Voice reply (TTS) | ✅ Gemini TTS → OGG/OPUS | ✅ | ✅ | ❌ |
| Send files | ✅ batch 10/message | ✅ | ✅ | ❌ |
| Receive attachments | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
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

> **Scheduler layer**: Built on [pardnchiu/go-scheduler](https://github.com/pardnchiu/go-scheduler), a self-maintained ecosystem package providing cron expression parsing, one-shot tasks, fsnotify hot-reload, and full output routing back to chat platforms.

***

### 8. Tool Ecosystem

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| MCP support | ✅ client | ✅ client | ✅ client + server | ✅ client | ❌ | ✅ client |
| Custom tools (auto-discovery) | ✅ AI-generated | ❌ | ✅ auto-creates skill | ❌ | ❌ | ❌ |
| API tool discovery (search-api → add) | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Tool registry (publish + install across machines) | ✅ pkg.agenvoy.com (Cloudflare Worker + R2 + D1, email verification + downgrade guard) | ⚠️ ClawHub (skills + plugins) | ⚠️ agentskills.io (skills only) | ❌ | ❌ | ❌ |
| Skill system | ✅ SKILL.md lazy-load | ✅ SKILL.md 5400+ community | ✅ SKILL.md agentskills.io | ✅ CLAUDE.md | ❌ | ❌ |
| Skill self-improvement (auto-fix on failure) | ✅ trace → rewrite → auto-commit | ❌ | ✅ | ❌ | ❌ | ❌ |
| Format reference as lazy-load tool | ✅ `format_chatbot` | ❌ | ❌ | ❌ | ❌ | ❌ |
| Document RAG (external knowledge base) | ✅ KuraDB (in-process vector + semantic/keyword) | ❌ (conversation memory only) | ❌ (conversation memory only) | ❌ | ❌ | ❌ |
| Media transcription STT | ✅ Gemini, 14 formats | ✅ Whisper/Gemini | ✅ faster-whisper (local) | ❌ | ❌ | ❌ |
| TTS voice output | ✅ Gemini TTS | ✅ ElevenLabs/Hume/MS | ✅ Edge TTS/ElevenLabs/OpenAI | ❌ | ❌ | ❌ |
| Computer use / browser | ✅ go-rod + Playwright MCP | ✅ Chrome CDP | ✅ browser CDP + computer-use (cua-driver) | ✅ beta | ❌ | ❌ |

> **Tool sandbox architecture**: Built on [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas) (Function as a Service). Each AI-generated tool runs as an isolated function unit with its own lifecycle and security boundary. The only FaaS-level sandbox design among all compared products.

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
| Cross-session memory | ⚠️ session-isolated by default, extensible | ✅ built-in cross-session | ✅ built-in cross-session | ⚠️ session-isolated by default, extensible | ⚠️ session-isolated | ⚠️ session-isolated |

> **Three-tier conversation memory**: (1) **Context** — latest 16 messages loaded into LLM context + periodic summary; (2) **ToriiDB** — self-developed embedded vector database ([pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)) for semantic similarity search on recent conversations; (3) **SQLite FTS5** — full-text archive via [pardnchiu/go-sqlite](https://github.com/pardnchiu/go-sqlite), dual-written on every message, never loses data even after history compaction.

***

### 10. Dependencies & Deployment

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Direct external dependencies | **12** | large (pnpm monorepo) | 30-40 core + 60+ optional | 50+ | 40+ | 40+ |
| Self-maintained ecosystem packages | 6 (go-bot / go-pkg / go-scheduler / ToriiDB / go-faas / KuraDB) | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go (static binary) | Node.js | Python | Node.js | Node.js + Rust | Node.js |
| Deployment | **single binary** | npm install | pip + docker/VPS | npm install | npm install | npm install |

***

### Where Agenvoy Stands

| Dimension | Detail |
|-----------|--------|
| **Clear advantages** | Single Go binary, 12 dependencies, self-maintained ecosystem (pardnchiu universe), dispatcher routing, Session Canvas, native platform UI (real buttons/modals), OTP verification, cross-session send to Telegram/Discord, API tool auto-discovery, format reference as lazy-load tool, local-only scheduler (no cloud) |
| **On par** | Telegram/Discord daemon, TTS/STT, scheduler output push, Skill system, MCP, browser automation, attachment handling, provider coverage (compat layer covers any OpenAI-compatible endpoint) |
| **Where competitors lead** | Hermes context compression engine (token-budget compaction), OpenClaw 24+ platforms, Hermes MCP server mode, Hermes local STT, OpenClaw/Hermes built-in cross-session memory, Claude Code Computer Use beta, Claude Code cloud cron/task |
| **Codex CLI** | Fewest features — CLI + TUI + OpenAI OAuth only, no daemon, no chat platforms, no scheduler |
