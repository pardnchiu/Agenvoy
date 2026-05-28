<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>讓 AI 真正為你工作——你的個人 AI 助理。</strong>
</p>

<p align="center">
  <strong>一句話，它自己長出工具——不需編程能力。</strong> Claude／GPT／Gemini 自動分工，在 Telegram、Discord、終端機隨叫隨到。
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/pardnchiu/agenvoy"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
  <a href="https://app.codecov.io/github/pardnchiu/agenvoy/tree/master"><img src="https://img.shields.io/codecov/c/github/pardnchiu/agenvoy/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
  <a href="../LICENSE"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
  <a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <a href="../README.md">English</a> · <strong>繁體中文</strong>
</p>

***

## Agenvoy 是什麼

一個跑在你自己機器上的**個人 AI Agent**。設定一次，從任何地方對話——Telegram、Discord、終端機 TUI、瀏覽器都行，跨所有 channel 共享同一份記憶、工具、技能。

為了想擁有自己的常駐助理、而非再訂閱另一個 SaaS 的人而設計。

## 它能做什麼

**讓 Agent 自己生成工具——不需編程能力。** 你只要描述需求，Agent 會寫 script 或串接 API、放進沙箱、註冊為工具。下次你再叫它，它就直接執行。

| Demo · 自動生成工具 | Demo · 以 Skill 為核心的排程器 |
| --- | --- |
| [![](https://i.ytimg.com/vi/WBCjLQ-nQFo/maxresdefault.jpg)](https://www.youtube.com/watch?v=WBCjLQ-nQFo) | [![](https://i.ytimg.com/vi/bO9AMrW3L9c/maxresdefault.jpg)](https://www.youtube.com/watch?v=bO9AMrW3L9c) |
| **Demo · 召喚 sub-agent 協作** | **Demo · 從 registry 安裝工具** |
| [![](https://i.ytimg.com/vi/wM3NU4ARz4w/maxresdefault.jpg)](https://www.youtube.com/watch?v=wM3NU4ARz4w) | [![](https://i.ytimg.com/vi/UrR5i7YAHRc/maxresdefault.jpg)](https://www.youtube.com/watch?v=UrR5i7YAHRc) |

開箱即用的還有：

- **在任何地方對話**——Telegram 內嵌按鈕、Discord 下拉選單／modal、終端機 TUI、瀏覽器 canvas。一個 daemon、所有介面。
- **生成圖片、語音回覆（TTS）、轉錄音訊／影片。**
- **自我排程**——說一句「每個工作天早上 8 點把 Hacker News 熱門摘要推到 Telegram」，Agent 會自動建立 cron 與推送管線。
- **依任務挑模型**——Claude 寫程式、Gemini 處理影音、GPT 做研究，自動路由。
- **語意搜尋你的檔案**——KuraDB 把本機文件／筆記做成向量索引（file → embedding），Agent 從你的知識庫回答，而非通用訓練資料。
- **跨 session 記憶**——過去對話依語意搜尋，而非只是關鍵字。
- **發布／安裝自訂工具**——透過 pkg.agenvoy.com registry 跨機器分享 AI 生成工具；上傳走 email 驗證碼 gate、版本嚴格遞增、安裝走單一 popup 並自動安裝依賴。

## 一鍵安裝

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

單一 Go binary 安裝至 `/usr/local/bin/agen`。macOS／Linux。不需 Node、Python、Docker。

> 在 MacBook 跑 daemon？跑一下 `sudo pmset -c sleep 0` 讓系統在接電源時不睡眠——避免 daemon 在 AC 電源下被掛起。

## 與其他工具比一比

只跟最接近的兩個對手比——同樣是「個人 AI Agent 框架 + daemon + 對話平台整合」的同類產品。

| 你想要的功能 | **Agenvoy** | OpenClaw | Hermes |
|---|---|---|---|
| 一鍵安裝、單一 binary | ✅ Go | ❌ pnpm monorepo | ❌ pip + docker |
| 同一個對話同時用 Claude + GPT + Gemini | ✅ dispatcher 自動路由 | ✅ 手動切換 | ✅ 手動切換 |
| 原生對話 UI（按鈕／選單／modal） | ✅ inline keyboard／select／modal | ⚠️ 純文字選項 | ⚠️ 純文字選項 |
| Agent 自己生成並保存工具 | ✅ FaaS 沙箱 script + API | ❌ | ⚠️ 僅 skill |
| Telegram／Discord 首次對話驗證 | ✅ 6 碼 OTP | ⚠️ pairing code（人工核准） | ❌ 僅 allowlist |
| 跨 session 推送（任一 session → chat） | ✅ `send_to_telegram_chat` / `send_to_discord_channel` | ❌ | ❌ |
| 對話中直接生成圖片 | ✅ gpt-image-2 | ❌ | ❌ |
| 原生文件 RAG（file → embedding） | ✅ KuraDB in-process（語意＋關鍵字） | ❌（需走 MCP） | ❌（需走 MCP） |

> 想看完整逐項對照？往下捲到 [**What makes it different**](#what-makes-it-different)。

***

# 給開發者

## What makes it different

- **Dispatcher 智能路由**——dispatcher 模型把每個任務路由給最合適的 worker（Claude 寫程式、Gemini 處理影音、GPT 做研究），而非強迫單一模型萬用。
- **Agent 會自己生成並持久化工具**——當工具不存在時，Agent 寫 script 或 API 到 `extensions/`，下次執行就以原生工具載入；同時支援 MCP server。
- **跨所有 channel 的單一 runtime**——Telegram、Discord、TUI、Web、cron 全部接上同一個 daemon；session、記憶、工具集共享，不是每個介面各自重建。

<details>
<summary><strong>Agenvoy 對主流產品：完整逐項對照</strong></summary>

### 1. 總覽

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| **語言** | Go | TypeScript | Python | TypeScript | Rust + TypeScript | TypeScript |
| **授權** | Apache 2.0 | MIT | MIT | Proprietary | Apache 2.0 | Apache 2.0 |
| **作者** | Individual (pardnchiu) | Community | NousResearch | Anthropic | OpenAI | Google |
| **主要用途** | 跨平台 AI Agent 框架 | 跨平台 AI Agent | 跨平台 AI Agent | 終端機程式助理 | 終端機程式助理 | 終端機程式助理 |
| **架構** | Daemon + TUI + Chat | Daemon + TUI + Chat | Daemon + TUI + Chat | CLI session | CLI session | CLI session |

---

### 2. AI Provider 支援

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Claude | ✅ | ✅ | ✅ | ✅ 限定 | ❌ | ❌ |
| OpenAI / GPT | ✅ | ✅ | ✅ | ❌ | ✅ 限定 | ❌ |
| Gemini | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ 限定 |
| Codex (OpenAI OAuth) | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ |
| GitHub Copilot | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| Nvidia NIM | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| OpenAI-compat | ✅ | ✅ Ollama／LM Studio | ✅ OpenRouter 200+ | ❌ | ❌ | ❌ |
| DeepSeek / Mistral / xAI | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Dispatcher 路由 | ✅ 專屬 dispatcher 模型 | ❌ | ❌ | ❌ | ❌ | ❌ |

---

### 3. Runtime 與前端

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| TUI | ✅ bubbletea | ✅ `openclaw tui` | ✅ React Ink | ✅ ink | ✅ | ✅ |
| CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| HTTP API / Web UI | ✅ gin | ❌ | ✅ Web Dashboard | ❌ | ❌ | ❌ |
| Daemon 模式 | ✅ 原生 `--daemon` | ✅ systemd／launchd | ✅ gateway daemon | ❌ | ❌ | ❌ |
| Session Canvas (HTML+SSE) | ✅ `update_page` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 具名 session | ✅ | ❌ | ✅ session picker | ❌ | ❌ | ❌ |

---

### 4. 對話平台整合

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Telegram | ✅ 原生 daemon | ✅ 原生 daemon | ✅ 原生 daemon | ⚠️ Channels MCP（需有 active session） | ❌ | ❌ |
| Discord | ✅ 原生 daemon | ✅ 原生 daemon | ✅ 原生 daemon | ⚠️ Channels MCP（需有 active session） | ❌ | ❌ |
| iMessage | ❌ | ✅ BlueBubbles | ✅ BlueBubbles | ⚠️ Channels MCP（僅 macOS） | ❌ | ❌ |
| WhatsApp / Slack / LINE | ❌ | ✅ 50+ 平台 | ✅ 20+ 平台 | ❌ | ❌ | ❌ |
| Always-on 收訊（不需 session） | ✅ daemon | ✅ | ✅ | ❌ | ❌ | ❌ |
| 跨 session 發送（任一 session → chat） | ✅ `send_to_telegram_chat` / `send_to_discord_channel` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 首次對話驗證 | ✅ 6 碼 OTP（crypto/rand） | ✅ pairing code（dmPolicy: pairing） | ❌（僅 allowlist） | ❌ | ❌ | ❌ |
| 原生平台 UI（按鈕／選單／modal） | ✅ inline keyboard / select menu / modal | ⚠️ 純文字選項 | ⚠️ 純文字選項 | ❌ | ❌ | ❌ |

> **平台層**：Agenvoy 的 Telegram 與 Discord 整合都建在 [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot) 上，獨立維護、開源。go-bot 封裝兩個平台的 bot 協定細節——Agenvoy 只實作業務邏輯，平台 API 層完全交由 go-bot 處理。

> **關鍵差異**：Claude Code Channels 需要 active session。OpenClaw 與 Hermes 有 daemon 但 in-chat 確認為純文字。Agenvoy 用原生平台 UI——Telegram inline keyboards 與 Discord select menus／modals。此外 Agenvoy 的 cross-session send 工具讓任一 session 類型（CLI／TUI／HTTP／scheduled script）都能推訊息到 Telegram／Discord——競爭者無此能力。

---

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
| Format reference（lazy-load tool） | ✅ `telegram_format` | ❌ | ❌ | ❌ |
| Scheduler 結果推送 | ✅ | ✅ | ✅ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_telegram_chat` | ❌ | ❌ | ❌ |
| 離線收訊（daemon） | ✅ | ✅ | ✅ | ❌ |

---

### 6. Discord 功能對照

| 功能 | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| 文字回覆 | ✅ | ✅ | ✅ | ✅ |
| 語音回覆（TTS） | ✅ Gemini TTS → OGG／OPUS | ✅ | ✅ | ❌ |
| 傳送檔案 | ✅ 一次最多 10 個／訊息 | ✅ | ✅ | ❌ |
| 接收使用者附件 | ✅ photo／doc／voice／video | ✅ | ✅ | ❌ |
| Tool confirm（互動式） | ✅ select menu button | ✅ `/model` picker | ⚠️ 純文字選項 | ❌ |
| ask_user（modal） | ✅ select／multi-select／modal | ⚠️ 限制多 | ⚠️ 純文字選項 | ❌ |
| Format reference（lazy-load tool） | ✅ `discord_format` | ❌ | ❌ | ❌ |
| Guild mention 守門 | ✅ | ✅ | ✅ | ❌ |
| Discord Markdown 感知 | ✅ 完整規格做 lazy-load tool | ⚠️ 部分 | ⚠️ 部分 | ❌ |
| 字數上限感知 | ✅ prompt 內 1600 字硬限 | ❌ | ❌ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_discord_channel` | ❌ | ❌ | ❌ |

---

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

---

### 8. 工具生態

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| MCP 支援 | ✅ client | ✅ client | ✅ client + server | ✅ client | ❌ | ✅ client |
| 自訂工具（script-tool-add） | ✅ AI 生成 | ❌ | ✅ 自動建立 skill | ❌ | ❌ | ❌ |
| API 工具自動探索（search-api → add） | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 工具 registry（跨機器發布／安裝） | ✅ pkg.agenvoy.com（Cloudflare Worker + R2 + D1，email 驗證碼 + 降版本封鎖） | ❌ | ⚠️ agentskills.io（僅 skill） | ❌ | ❌ | ❌ |
| Skill 系統 | ✅ SKILL.md lazy-load | ✅ SKILL.md 5400+ 社群 | ✅ SKILL.md agentskills.io | ✅ CLAUDE.md | ❌ | ❌ |
| Format reference 作為 lazy-load tool | ✅ `telegram_format` / `discord_format` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 圖像生成 | ✅ DALL-E／Codex Image | ❌ | ❌ | ❌ | ❌ | ❌ |
| 文件 RAG（外部知識庫） | ✅ KuraDB（in-process 向量＋語意／關鍵字） | ❌（僅對話記憶向量） | ❌（僅對話記憶 FTS5） | ❌ | ❌ | ❌ |
| 媒體 STT 轉錄 | ✅ Gemini，14 種格式 | ✅ Whisper／Gemini | ✅ faster-whisper（本機） | ❌ | ❌ | ❌ |
| TTS 語音輸出 | ✅ Gemini TTS | ✅ ElevenLabs／Hume／MS | ✅ Edge TTS／ElevenLabs／OpenAI | ❌ | ❌ | ❌ |
| Computer use／browser | ✅ go-rod + Playwright MCP | ✅ Chrome CDP | ✅ Playwright（Chromium／Firefox） | ✅ beta | ❌ | ❌ |

> **工具沙箱架構**：Agenvoy 的 Python／JavaScript／API 自訂工具介面建在 [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas)（Function as a Service）概念上。每個 AI 生成的工具以隔離函式單元運行，各自有生命週期與安全邊界。在所有對照產品中，唯一的 FaaS 級工具沙箱設計。

---

### 9. 記憶系統

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| Instruction file 系統 | ✅ SKILL.md | ✅ SKILL.md | ✅ SKILL.md | ✅ CLAUDE.md | ❌ | ❌ |
| 對話歷史搜尋 | ✅ ToriiDB 向量搜尋 | ✅ SQLite 向量 | ✅ SQLite FTS5 | ❌ | ❌ | ❌ |
| 外部文件 RAG（原生 in-process） | ✅ KuraDB（語意＋關鍵字，OpenAI embeddings） | ❌（需走 MCP） | ❌（需走 MCP） | ❌ | ❌ | ❌ |
| 錯誤記憶 | ✅ ToriiDB | ❌ | ❌ | ❌ | ❌ | ❌ |
| Action log | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 長期持久記憶 | ⚠️ ToriiDB 基礎已就緒 | ✅ Wiki-style MEMORY.md | ✅ MEMORY.md + USER.md | ⚠️ CLAUDE.md 手動 | ❌ | ❌ |
| 跨 session 記憶 | ⚠️ 預設 session 隔離，可外接記憶擴展 | ✅ 內建跨 session | ✅ 內建跨 session | ⚠️ 預設 session 隔離，可外接記憶擴展 | ⚠️ 預設 session 隔離 | ⚠️ 預設 session 隔離 |

> **ToriiDB** 是 Agenvoy 生態中自研的內嵌式向量資料庫（[pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)）。不需外部服務，in-process 運行。Agenvoy 以 ToriiDB 作為記憶基礎設施，目前驅動語意對話搜尋與錯誤記憶，並作為未來長期跨 session 記憶擴展的基底。

---

### 10. 依賴與部署

| | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code** | **Codex CLI** | **Gemini CLI** |
|--|--|--|--|--|--|--|
| 直接外部依賴 | **12** | 大量（pnpm monorepo） | 30–40 core + 60+ optional | 50+ | 40+ | 40+ |
| 自家維護的 ecosystem package | 6（go-bot／go-pkg／go-scheduler／ToriiDB／go-faas／KuraDB） | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go（靜態 binary） | Node.js | Python | Node.js | Node.js + Rust | Node.js |
| 部署 | **單一 binary** | npm install | pip + docker／VPS | npm install | npm install | npm install |

---

### Agenvoy 的定位

| 維度 | 細節 |
|-----------|--------|
| **明確優勢** | 單一 Go binary、12 個依賴、自家 ecosystem（pardnchiu universe）、dispatcher 路由、Session Canvas、原生平台 UI（真按鈕／modal）、OTP 驗證、從任一 session 跨送 Telegram／Discord、API 工具自動探索、圖像生成、format reference 做 lazy-load tool、純本機 scheduler（不需雲端） |
| **與競爭者相當** | Telegram／Discord daemon、TTS／STT、scheduler 結果推送、Skill 系統、MCP、瀏覽器自動化、附件接收 |
| **競爭者領先處** | OpenClaw 50+ 平台、Hermes MCP server 模式、Hermes 本機 STT、OpenClaw／Hermes 內建跨 session 記憶、Claude Code Computer Use beta、Claude Code 雲端 cron／task |
| **Codex CLI** | 功能最少——僅 CLI + TUI + OpenAI OAuth，無 daemon、無對話平台、無 scheduler |

</details>

<details>
<summary><strong>CLI 指令</strong></summary>

> 以 `agen <sub>` 執行。repo Makefile 內有 `make <sub>` wrapper 供開發用。

| 指令 | 說明 |
|---|---|
| `agen` | 進入互動式 TUI；若 daemon 未啟動則自動 fork（HTTP + Discord + Telegram + scheduler + summary cron）。 |
| `agen cli <input>` | 一次性 agent run；每個 tool call 都會請求確認。 |
| `agen run <input>` | 一次性 agent run；自動同意所有 tool call。 |
| `agen stop` | 停止運行中的 daemon（SIGTERM 5s grace → SIGKILL → 清除 `runtime.uid`）。 |
| `agen update` | 抓最新 release、重編、停 daemon——重新 attach 即載入新 binary。 |
| `agen model {add\|remove\|list\|dispatcher\|reasoning}` | 管理 provider／worker model、選 dispatcher、設 reasoning 等級。 |
| `agen mcp {list\|add\|remove}` | 管理 MCP server（stdio／HTTP），支援 global 與 per-session scope。 |
| `agen session {new\|switch\|config} [name]` | 管理 CLI session；不帶參數的 `switch` / `config` 會開互動式 picker。 |

</details>

<details>
<summary><strong>TUI slash 指令</strong></summary>

> 在 `agen` TUI 輸入框內可用。輸入 `/` 篩選；popup 指令會 cleanly 回到輸入框。

| 指令 | 說明 |
|---|---|
| `/switch` | 透過 picker 切換 active session（current session 預選）。 |
| `/new [name]` | 建立新 session；可選 name 釘進 registry。Name 與既有 session 衝突檢查；衝突即 abort。 |
| `/bot` | 編輯當前 session 的 bot，走兩個串接 popup：name textfield（與其他 session 衝突檢查；衝突即 abort）→ description textarea（`Ctrl+S` 確認、`Enter` 換行、`Esc` 取消）。 |
| `/model [global\|session]` | Scope picker；`global` → `[add, remove]`（registry），`session` → 從已設定 model 挑一個。Inline arg 跳過 scope popup。 |
| `/mcp [add\|remove]` | Action picker；`add` 走串接 popup 表單（name → transport → command/args/env 或 url/headers → scope → 選擇性 session pick），`remove` 列出 global 與 session scope 內的 server。改動需重啟 daemon 生效。Inline arg 跳過 action popup。 |
| `/dispatcher` | 從 `cfg.Models` 透過 popup 挑 dispatcher model。無 inline arg。 |
| `/reasoning [global\|session]` | 為 dispatcher（global）或當前 session 挑 `low`／`medium`／`high`。Inline arg 跳過 scope popup。 |
| `/discord [enable\|disable]` | 切換 Discord bot 連線（token 輸入、驗證、keychain 寫入、daemon reload 全在 TUI 內完成）。Inline arg 直接切換不開 popup。 |
| `/telegram [enable\|disable]` | 切換 Telegram bot 連線（與 `/discord` 同樣 in-TUI popup chain；第一個 chat 來訊息時必須通過 in-chat 驗證碼）。Inline arg 直接切換不開 popup。 |
| `/kuradb [enable\|disable]` | 切換 KuraDB RAG 服務。`enable` 透過 `tea.ExecProcess` 跑 `install.sh`（sudo TTY 還給 child），收 `OPENAI_API_KEY`（寫入 keychain），寫入 `kuradb_enabled=true`——daemon 透過 fsnotify spawn child + 寫 endpoint 檔。`disable` 移除 `/usr/local/bin/kura` 並清 flag。Inline arg 直接切換不開 popup。 |
| `/cron [add\|remove\|edit]` | 管理週期排程。`add` 開多行 requirement textarea → dispatch `/scheduler-skill-creator <requirement>`（缺少 when／what 時透過 `ask_user` 問）。`remove` 列出 cron → 確認 popup → `runtime.RemoveCron` + 把 skill dir 丟進 trash。`edit` 列出 cron → requirement textarea → agent 選擇 `patch_cron` 或重寫 SKILL.md body。Inline arg 跳過 action popup。 |
| `/task [add\|remove\|edit]` | 管理 one-shot 排程任務（鏡像 `/cron`；用 `add_task`／`patch_task`／`remove_task`）。Picker 顯示 `<YYYY-MM-DD HH:MM>  <skill>`。 |
| `/sched-<name>` | 內聯執行已存在的 scheduler skill body（手動觸發）。出現在 `/` picker 底部正常 skill 之後；label 以 warn-purple 標示為「呼叫式」。Dispatch 會在 body 前包一段「execute, do NOT activate scheduler-skill-creator」前言。 |
| `/mode [cli\|web]` | 在 `cli`（TUI 渲染）與 `web`（瀏覽器頁面）間切換。Inline arg 直接切換不開 popup。 |
| `/update` | 確認 popup → 透過 `tea.ExecProcess` 跑 `agen stop && agen update` → 退出 TUI。 |
| `/history` | 重新載入可見 transcript——清螢幕、reprint header、從 session 的 `action.log` 渲染最近 100 筆。 |
| `/log` | 在 `$PAGER`（fallback `less -Rf +G`，跳到底）開啟 raw `action.log`。`\x1F` markers 會展開回 newline 以利閱讀。 |
| `/clear` | 只清當前視窗顯示——類似 terminal `clear`；對話記憶不動。 |
| `/exit`, `/quit` | 退出 TUI（daemon 繼續運行；以 `agen` 重新 attach）。 |

</details>

<details>
<summary><strong>內建工具</strong></summary>

> 工具依需求自動載入；stub name 先出現，full schema 在使用時才啟用。參數與路由請見 [Tools wiki](https://github.com/pardnchiu/Agenvoy/blob/master/wiki/Tools.zh.md)。

| Tool | 說明 |
|---|---|
| **File** |  |
| `read_file` | 讀取 text、PDF、DOCX、PPTX、CSV／TSV、image 檔案。 |
| `write_file` | 寫入內容到檔案，若已存在則覆蓋。 |
| `patch_file` | 取代檔案內精確字串。 |
| `list_files` | 列出目錄項目；`recursive=true` walk 子樹檔案。 |
| `glob_files` | 在目錄內以 glob pattern 找檔。 |
| `search_files` | 以 RE2 regex 在目錄內搜尋檔案內容。 |
| **Web** |  |
| `fetch_page` | 抓取網頁，回傳 Markdown 內容。 |
| `save_page_to_file` | 抓取網頁並把內容存到本機檔案。 |
| `search_web` | 透過 DuckDuckGo Lite 搜尋網頁；回傳前 10 筆。 |
| `fetch_google_rss` | 搜尋 Google News RSS，回傳標題、摘要、連結。 |
| `fetch_yahoo_finance` | 查詢 Yahoo Finance 報價與 K 線（OHLCV）。 |
| `fetch_youtube_transcript` | 帶時間戳轉錄 YouTube 影片。*(需 gemini)* |
| `transcribe_media` | 轉錄本機音訊／影片（ogg、mp3、wav、m4a、flac、aac、mp4、mov、webm、mpeg、3gp、…），上限 20 MiB。*(需 gemini)* |
| `send_http_request` | 對指定 URL 送 HTTP request。 |
| **Shell** |  |
| `run_command` | 以 argv 執行 binary；回傳合併的 stdout／stderr。 |
| **Render** |  |
| `update_page` | 覆寫當前 session 的渲染 HTML 頁；分頁自動 reload。 |
| `generate_image` | 透過 gpt-image-2 生圖（size 與 quality 由使用者選）。*(需 codex)* |
| **Channel** |  |
| `list_telegram_chat` | 列出已授權 Telegram chat（id + name）。*(需 telegram)* |
| `send_to_telegram_chat` | 依 chat_id 送 HTML 格式訊息到已授權 Telegram chat。*(需 telegram)* |
| `telegram_format` | 回傳 Telegram HTML formatting reference（允許 tag、escape 規則、檔案／語音 markers）。*(需 telegram)* |
| `list_discord_channel` | 列出已授權 Discord channel（id + name）。*(需 discord)* |
| `send_to_discord_channel` | 依 channel_id 送 markdown 格式訊息到已授權 Discord channel。*(需 discord)* |
| `discord_format` | 回傳 Discord markdown formatting reference（允許 markdown、特殊 token、檔案／語音 markers）。*(需 discord)* |
| **Calc** |  |
| `calculate` | 求值數學表達式並回傳精確結果。 |
| **Discovery** |  |
| `list_tools` | 列出當前所有可用的內建與動態載入工具。 |
| `search_tools` | 依關鍵字搜尋工具並注入命中項到 request。 |
| `activate_skill` | 依精確名稱抓 skill 的 reference material。 |
| **Interactive** |  |
| `ask_user` | 問 user 一個或多個問題並回傳答案。 |
| `store_secret` | 以遮罩輸入向 user 索取 secret，持久化到系統 keychain。 |
| **Memory** |  |
| `search_conversation_history` | 依關鍵字與語意相似度搜尋 session 過去訊息。 |
| `search_error_memory` | 語意搜尋過去 tool-error 記錄；命中即續期 3 個月 TTL。 |
| `read_error_memory` | 依 hash 抓特定 tool-error 記錄。 |
| `remember_error` | 持久化 tool-error 記錄供日後查找。 |
| **RAG（KuraDB）** |  |
| `rag_list_db` | 列出可用的 KuraDB 資料庫（如 notes、inbox、code）。*(需 kuradb)* |
| `rag_search_keyword` | 透過 gse 分詞對 KuraDB 資料庫做關鍵字搜尋。*(需 kuradb)* |
| `rag_search_semantic` | 透過 OpenAI embeddings 對 KuraDB 資料庫做語意搜尋。*(需 kuradb)* |
| **Agent** |  |
| `invoke_subagent` | 在內部 subagent session 跑一個 subtask 並回傳最終文字。 |
| `invoke_external_agent` | 呼叫一個外部 CLI agent（codex／copilot／claude／gemini）取得第二意見。 |
| `cross_review_with_external_agents` | 平行讓所有可用外部 agent 交叉 review 一個結果。 |
| `review_result` | 將結果對照原輸入做 review，回傳問題與改進建議。 |
| **Scheduler** |  |
| `add_task` | 把既有 scheduler skill 綁到一個特定時間單次觸發（`+5m`／`HH:MM`／`YYYY-MM-DD HH:MM`／RFC3339）。 |
| `add_cron` | 把既有 scheduler skill 綁到 5 欄位 cron 表達式週期觸發。 |
| `patch_task` / `patch_cron` | 依 skill 名稱重排 task／cron（只改時間，不動綁定的 skill body）。 |
| `remove_task` / `remove_cron` | 依 skill 名稱取消 task／cron；綁定的 scheduler skill dir 被搬到 `.Trash/`。 |
| **Skill Git** |  |
| `skill_git_commit` / `skill_git_log` / `skill_git_rollback` | Commit、列出、或 rollback `~/.config/agenvoy/skills` 的 git 歷史。 |

動態工具家族（自動註冊，未列出）：來自已設定 MCP server 的 `mcp__<server>__<tool>`、來自 `extensions/apis/*.json` 的 `api_<name>`、來自 `extensions/scripts/<name>/` 的 `script_<name>`。

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

本專案以 [Apache License 2.0](../LICENSE) 授權。

## Contributor

歡迎 [開 issue](https://github.com/pardnchiu/agenvoy/issues/new) 分享想法。

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

曲線往上走，就是我們想要的訊號。按 ★ 推一把。

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
