# Agenvoy Wiki

**Agenvoy** makes AI actually work for you — a personal AI assistant runtime that is always-on, model-agnostic, and self-improving. A single Go daemon talks to multiple LLM providers (Claude / GPT / Gemini auto-routed), reaches you through Telegram / Discord / TUI / browser, and lets the agent build & persist its own tools as you ask.

## Highlights

- **Nine LLM providers** — Claude, OpenAI, Codex (OAuth), Gemini, GitHub Copilot, Nvidia NIM, DeepSeek, xAI Grok, Compat (any OpenAI-compatible endpoint, Zed-style `/v1` URL)
- **Dispatcher-based routing** — a dispatcher LLM routes each task to the best-fit worker (Claude for coding, Gemini for video, GPT for research)
- **Three-pass concurrent tool dispatch** — read tools fan out concurrently; write tools stay serial for safety
- **Multi-layer memory** — rolling summary (incremental, timestamp-cursored) + 16-message recent history + keyword/semantic dual search + cross-session error memory with 90-day TTL
- **Native document RAG** — KuraDB in-process child process (`list_rag` / `search_rag`), enabled via `/feature kuradb` in the TUI
- **Skill system** — loadable markdown skill packs triggered by `/skill-name` or `run_skill`; scheduler skills isolated under `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/`
- **OS sandbox** — Linux bubblewrap / macOS sandbox-exec; tools execute in isolation
- **MCP client** — stdio + HTTP/SSE; tools auto-inject as `mcp__<server>__<tool>`
- **Chat platform integration** — Telegram (6-digit OTP first-contact verification) + Discord (native select menus / modals); cross-session push via `send_to_chatbot`
- **Voice & attachments** — `[SEND_VOICE:text]` to Gemini TTS (OGG/OPUS); inbound attachments saved to download dir
- **Sub-agents & external agents** — `invoke_subagent` (in-process) + `invoke_external_agent` (codex / copilot / claude / gemini CLI) + `cross_review_with_external_agents` (parallel review)
- **Scheduler** — cron / one-shot tasks, fsnotify hot-reload, output push back to Telegram/Discord
- **Send-timeout 3-layer system** — Transport `ResponseHeaderTimeout=10s`, `Client.Timeout` 5m / 10m (SSE), exec layer `AgentSendTimeout` 600s with retry

## Source

- Repository: [pardnchiu/Agenvoy](https://github.com/pardnchiu/Agenvoy)
