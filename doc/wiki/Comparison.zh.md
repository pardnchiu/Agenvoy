# 給開發者

## What makes it different

- **Dispatcher 智能路由**——dispatcher 模型把每個任務路由給最合適的 worker（Claude 寫程式、Gemini 處理影音、GPT 做研究），而非強迫單一模型萬用。
- **Agent 會自己生成並持久化工具**——當工具不存在時，Agent 寫 script 或 API 到 `extensions/`，下次執行就以原生工具載入；同時支援 MCP server。
- **跨所有 channel 的單一 runtime**——Telegram、Discord、TUI、Web、cron 全部接上同一個 daemon；session、記憶、工具集共享，不是每個介面各自重建。

## Agenvoy 對主流產品：完整逐項對照

### 1. 總覽

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| **語言** | Go | TypeScript | Python | TypeScript | Rust + TypeScript | TypeScript |
| **授權** | Apache 2.0 | MIT | MIT | Proprietary | Apache 2.0 | Apache 2.0 |
| **作者** | Individual (pardnchiu) | Community | NousResearch | Anthropic | OpenAI | Google |
| **主要用途** | 跨平台 AI Agent 框架 | 跨平台 AI Agent | 跨平台 AI Agent | 終端機程式助理 | 終端機程式助理 | 終端機程式助理 |
| **架構** | Daemon + TUI + Chat | Daemon + TUI + Chat | Daemon + TUI + Chat | CLI session | CLI session | CLI session |

***

### 2. AI Provider 支援

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Claude | ✅ | ✅ | ✅ | ✅ 限定 | ❌ | ❌ |
| OpenAI / GPT | ✅ | ✅ | ✅ | ❌ | ✅ 限定 | ❌ |
| Gemini | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ 限定 |
| Codex (OpenAI OAuth) | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| GitHub Copilot | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Nvidia NIM | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| OpenAI-compat | ✅ | ✅ Ollama／LM Studio | ✅ OpenRouter 200+ | ❌ | ❌ | ❌ |
| DeepSeek | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| xAI (Grok) | ✅ API key | ✅ | ✅ OAuth + API key | ❌ | ❌ | ❌ |
| Mistral | ❌ | ✅ | ⚠️ 透過 OpenRouter（無專屬） | ❌ | ❌ | ❌ |
| Dispatcher 路由 | ✅ 專屬 dispatcher 模型 | ❌ | ❌ | ❌ | ❌ | ❌ |

***

### 3. Runtime 與前端

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| TUI | ✅ bubbletea | ✅ `openclaw tui` | ✅ React Ink | ✅ ink | ✅ | ✅ |
| CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| HTTP API / Web UI | ✅ gin | ✅ dashboard / webchat | ✅ Web Dashboard | ❌ | ❌ | ❌ |
| Daemon 模式 | ✅ 原生 `--daemon` | ✅ systemd／launchd | ✅ gateway daemon | ❌ | ❌ | ❌ |
| Session Canvas (HTML+SSE) | ✅ `render_page` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 具名 session | ✅ | ⚠️ workspaces / per-agent sessions | ✅ session picker | ❌ | ❌ | ❌ |

***

### 4. 對話平台整合

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Telegram | ✅ 原生 daemon | ✅ 原生 daemon | ✅ 原生 daemon | ⚠️ Channels MCP（需有 active session） | ❌ | ❌ |
| Discord | ✅ 原生 daemon | ✅ 原生 daemon | ✅ 原生 daemon | ⚠️ Channels MCP（需有 active session） | ❌ | ❌ |
| iMessage | ❌ | ✅ BlueBubbles | ✅ BlueBubbles | ⚠️ Channels MCP（僅 macOS） | ❌ | ❌ |
| LINE | ⚠️ alpha（[linebot 分支](https://github.com/pardnchiu/Agenvoy/tree/linebot)） | ✅ | ✅ | ❌ | ❌ | ❌ |
| WhatsApp / Slack | ❌ | ✅ 24+ 平台 | ✅ 24+ 平台 | ❌ | ❌ | ❌ |
| Always-on 收訊（不需 session） | ✅ daemon | ✅ | ✅ | ❌ | ❌ | ❌ |
| 跨 session 發送（任一 session → chat） | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ | ❌ | ❌ |
| 首次對話驗證 | ✅ 6 碼 OTP（crypto/rand） | ✅ pairing code（dmPolicy: pairing） | ✅ pairing code（`gateway/pairing.py`） | ❌ | ❌ | ❌ |
| 原生平台 UI（按鈕／選單／modal） | ✅ inline keyboard / select menu / modal | ⚠️ 純文字選項 | ⚠️ 純文字選項 | ❌ | ❌ | ❌ |

> **平台層**：Agenvoy 的 Telegram 與 Discord 整合都建在 [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot) 上，獨立維護、開源。go-bot 封裝兩個平台的 bot 協定細節——Agenvoy 只實作業務邏輯，平台 API 層完全交由 go-bot 處理。

> **關鍵差異**：Claude Code Channels 需要 active session。OpenClaw 與 Hermes 有 daemon 但 in-chat 確認為純文字。Agenvoy 用原生平台 UI——Telegram inline keyboards 與 Discord select menus／modals。此外 Agenvoy 的 cross-session send 工具讓任一 session 類型（CLI／TUI／HTTP／scheduled script）都能推訊息到特定 Telegram／Discord chat——競爭者僅部分支援（如 Hermes 的 `send_message` 僅限自身 gateway scope）。

***

### 5. Telegram 功能對照

| 功能 | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| 文字回覆 | ✅ | ✅ | ✅ | ✅ |
| 語音回覆（TTS） | ✅ Gemini TTS → OGG | ✅ ElevenLabs／Hume | ✅ Edge TTS／ElevenLabs | ❌ |
| 傳送檔案 | ✅ `[SEND_FILE:]` | ✅ | ✅ | ❌ |
| 接收使用者附件 | ✅ photo／doc／voice／video | ✅ | ✅ | ❌ |
| 語音轉文字（STT） | ✅ Gemini，14 種格式 | ✅ Whisper／Gemini | ✅ faster-whisper（本機） | ❌ |
| Tool confirm（互動式） | ✅ 原生 inline keyboard | ⚠️ 文字確認提示 | ⚠️ 純文字選項 | ❌ |
| ask_user（picker） | ✅ 原生 button／modal | ⚠️ `/models` picker | ⚠️ 純文字選項，上限 4 | ❌ |
| Format reference（lazy-load tool） | ✅ `format_chatbot` | ❌ | ❌ | ❌ |
| Scheduler 結果推送 | ✅ | ✅ | ✅ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ |
| 離線收訊（daemon） | ✅ | ✅ | ✅ | ❌ |

***

### 6. Discord 功能對照

| 功能 | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| 文字回覆 | ✅ | ✅ | ✅ | ✅ |
| 語音回覆（TTS） | ✅ Gemini TTS → OGG／OPUS | ✅ | ✅ | ❌ |
| 傳送檔案 | ✅ 一次最多 10 個／訊息 | ✅ | ✅ | ❌ |
| 接收使用者附件 | ✅ photo／doc／voice／video | ✅ | ✅ | ❌ |
| Tool confirm（互動式） | ✅ select menu button | ✅ `/model` picker | ⚠️ 純文字選項 | ❌ |
| ask_user（modal） | ✅ select／multi-select／modal | ⚠️ 限制多 | ⚠️ 純文字選項 | ❌ |
| Format reference（lazy-load tool） | ✅ `format_chatbot` | ❌ | ❌ | ❌ |
| Guild mention 守門 | ✅ | ✅ | ✅ | ❌ |
| Discord Markdown 感知 | ✅ 完整規格做 lazy-load tool | ⚠️ 部分 | ⚠️ 部分 | ❌ |
| 字數上限感知 | ✅ prompt 內 1600 字硬限 | ❌ | ❌ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_chatbot` | ❌ | ⚠️ `send_message` tool | ❌ |

***

### 7. Scheduler

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Cron job | ✅ SKILL.md + cron | ✅ 內建 | ✅ 內建 | ✅ 雲端輔助 cron／task | ❌ | ❌ |
| One-shot 任務 | ✅ | ✅ `at` 格式 | ✅ 自然語言 | ✅ 雲端輔助 | ❌ | ❌ |
| TUI CRUD | ✅ | ✅ `openclaw cron` | ✅ `cronjob` tool | ❌ | ❌ | ❌ |
| fsnotify 熱載入 | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 推送結果到 Telegram／Discord | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| AI 工具管理（add／list／remove） | ✅ | ❌ | ✅ `cronjob` tool | ❌ | ❌ | ❌ |
| 本機執行（不需雲端） | ✅ | ✅ | ✅ | ❌ 需雲端 | ❌ | ❌ |

> **Scheduler 層**：Agenvoy 的 scheduler 建在 [pardnchiu/go-scheduler](https://github.com/pardnchiu/go-scheduler) 上，自家維護的 ecosystem package，提供 cron 表達式解析、one-shot 任務、fsnotify 熱載入，以及完整把結果路由回對話平台。

***

### 8. 工具生態

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| MCP 支援 | ✅ client | ✅ client | ✅ client + server | ✅ client | ❌ | ✅ client |
| 自訂工具（auto-discovery） | ✅ AI 生成 | ❌ | ✅ 自動建立 skill | ❌ | ❌ | ❌ |
| API 工具自動探索（search-api → add） | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 工具 registry（跨機器發布／安裝） | ✅ pkg.agenvoy.com（Cloudflare Worker + R2 + D1，email 驗證碼 + 降版本封鎖） | ⚠️ ClawHub（skills + plugins） | ⚠️ agentskills.io（僅 skill） | ❌ | ❌ | ❌ |
| Skill 系統 | ✅ SKILL.md lazy-load | ✅ SKILL.md 5400+ 社群 | ✅ SKILL.md agentskills.io | ✅ CLAUDE.md | ❌ | ❌ |
| Skill 自我改進（失敗時自動修正） | ✅ trace → rewrite → auto-commit | ❌ | ✅ | ❌ | ❌ | ❌ |
| Format reference 作為 lazy-load tool | ✅ `format_chatbot` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 文件 RAG（外部知識庫） | ✅ KuraDB（in-process 向量＋語意／關鍵字） | ❌（僅對話記憶向量） | ❌（僅對話記憶 FTS5） | ❌ | ❌ | ❌ |
| 媒體 STT 轉錄 | ✅ Gemini，14 種格式 | ✅ Whisper／Gemini | ✅ faster-whisper（本機） | ❌ | ❌ | ❌ |
| TTS 語音輸出 | ✅ Gemini TTS | ✅ ElevenLabs／Hume／MS | ✅ Edge TTS／ElevenLabs／OpenAI | ❌ | ❌ | ❌ |
| Computer use／browser | ✅ go-rod + Playwright MCP | ✅ Chrome CDP | ✅ browser CDP + computer-use（cua-driver） | ✅ beta | ❌ | ❌ |

> **工具沙箱架構**：Agenvoy 的 Python／JavaScript／API 自訂工具介面建在 [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas)（Function as a Service）概念上。每個 AI 生成的工具以隔離函式單元運行，各自有生命週期與安全邊界。在所有對照產品中，唯一的 FaaS 級工具沙箱設計。

***

### 9. 記憶系統

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Instruction file 系統 | ✅ SKILL.md | ✅ SKILL.md | ✅ SKILL.md | ✅ CLAUDE.md | ❌ | ❌ |
| 對話歷史搜尋 | ✅ 三層：上下文 + ToriiDB 向量 + SQLite FTS5 | ✅ LanceDB 向量 | ✅ SQLite FTS5 | ❌ | ❌ | ❌ |
| 外部文件 RAG（原生 in-process） | ✅ KuraDB（語意＋關鍵字，OpenAI embeddings） | ❌（需走 MCP） | ❌（需走 MCP） | ❌ | ❌ | ❌ |
| 錯誤記憶 | ✅ ToriiDB | ❌ | ❌ | ❌ | ❌ | ❌ |
| Action log | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 長期持久記憶 | ✅ SQLite 全文歸檔（雙寫，資料永不遺失） | ✅ Wiki-style MEMORY.md | ✅ MEMORY.md + USER.md | ⚠️ CLAUDE.md 手動 | ❌ | ❌ |
| 跨 session 記憶 | ⚠️ 預設 session 隔離，可外接記憶擴展 | ✅ 內建跨 session | ✅ 內建跨 session | ⚠️ 預設 session 隔離，可外接記憶擴展 | ⚠️ 預設 session 隔離 | ⚠️ 預設 session 隔離 |

> **三層對話記憶**：(1) **上下文** — 最新 16 筆訊息直接載入 LLM context + 定期 summary；(2) **ToriiDB** — 自研內嵌式向量資料庫（[pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)），對近期對話做語意相似搜尋；(3) **SQLite FTS5** — 透過 [pardnchiu/go-sqlite](https://github.com/pardnchiu/go-sqlite) 的全文歸檔，每筆訊息雙寫，即使 history 裁剪後資料也不遺失。`search_chat_history` 依 `mode` 路由：`semantic` → ToriiDB、`keyword` → SQLite FTS5。

***

### 10. 依賴與部署

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| 直接外部依賴 | **12** | 大量（pnpm monorepo） | 30–40 core + 60+ optional | 50+ | 40+ | 40+ |
| 自家維護的 ecosystem package | 6（go-bot／go-pkg／go-scheduler／ToriiDB／go-faas／KuraDB） | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go（靜態 binary） | Node.js | Python | Node.js | Node.js + Rust | Node.js |
| 部署 | **單一 binary** | npm install | pip + docker／VPS | npm install | npm install | npm install |

***

### Agenvoy 的定位

| 維度 | 細節 |
|-----------|--------|
| **明確優勢** | 單一 Go binary、12 個依賴、自家 ecosystem（pardnchiu universe）、dispatcher 路由、Session Canvas、原生平台 UI（真按鈕／modal）、OTP 驗證、從任一 session 跨送 Telegram／Discord、API 工具自動探索、format reference 做 lazy-load tool、純本機 scheduler（不需雲端） |
| **與競爭者相當** | Telegram／Discord daemon、TTS／STT、scheduler 結果推送、Skill 系統、MCP、瀏覽器自動化、附件接收、provider 涵蓋範圍（compat 層支援任何 OpenAI 相容端點） |
| **競爭者領先處** | Hermes context compression engine（token-budget compaction：head preservation + middle-turn summarization + iterative recompression，對比 Agenvoy 的 reactive trim-only）、OpenClaw 24+ 平台、Hermes MCP server 模式、Hermes 本機 STT、OpenClaw／Hermes 內建跨 session 記憶、Claude Code Computer Use beta、Claude Code 雲端 cron／task |
| **Codex CLI** | 功能最少——僅 CLI + TUI + OpenAI OAuth，無 daemon、無對話平台、無 scheduler |
