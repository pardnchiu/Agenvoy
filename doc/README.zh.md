> [!NOTE]
> 此 README 由 [SKILL](https://github.com/pardnchiu/skill-readme-generate) 生成，英文版請參閱 [這裡](../README.md)。<br>
> 測試由 [SKILL](https://github.com/pardnchiu/skill-coverage-generate) 生成。

***

<p align="center">
<picture style="margin-down: 1rem">
<img src="./logo.svg" alt="Agenvoy" width="320">
</picture>
</p>

<p align="center">
<strong>一個指令，善用各種模型優勢，指派多個模型分工協作。</strong>
</p>

<p align="center">
Go-native runtime · Dispatcher 將每個步驟派給最適合的模型 · Subagent 在同一 process 內協作
</p>

<p align="center">
<a href="https://pkg.go.dev/github.com/pardnchiu/agenvoy"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
<a href="https://app.codecov.io/github/pardnchiu/agenvoy/tree/master"><img src="https://img.shields.io/codecov/c/github/pardnchiu/agenvoy/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
<a href="LICENSE"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
<a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

***

## 一鍵安裝

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

一行指令、單一 binary 落在 `/usr/local/bin/agen`，macOS／Linux 通用。

想在 MacBook 上跑 daemon 的話，接電源時跑 `sudo pmset -c sleep 0` 維持系統喚醒，避免 AC 模式下 daemon 被休眠中斷。

## 特點

- **Dispatcher-based 智能路由** —— Dispatcher model 把每個任務派給最合適的 worker（寫程式找 Claude、看影片找 Gemini、查資料找 GPT），不是同一個 model 硬扛。
- **Agent 自造工具並持久化** —— 缺工具，agent 自己寫 script / API 存進 `extensions/`，下次以原生 tool 形式自動載入；同時相容 MCP server。
- **多通道共用 runtime** —— Telegram、Discord、TUI、Web、cron 都接同一個 daemon，session、記憶、工具集打通，不必各自重建。

<details>
<summary><strong>Agenvoy 與主流產品：完整功能對比</strong></summary>

### 1. 概觀

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| **語言** | Go | TypeScript | TypeScript | TypeScript | TypeScript | Python |
| **授權** | Apache 2.0 | Proprietary | Apache 2.0 | Apache 2.0 | MIT | MIT |
| **作者** | 個人（pardnchiu） | Anthropic | OpenAI | Google | 社群 | NousResearch |
| **主要定位** | 跨平台 AI Agent 框架 | 終端機 coding 助手 | 終端機 coding 助手 | 終端機 coding 助手 | 跨平台 AI Agent | 跨平台 AI Agent |
| **架構** | Daemon + TUI + Chat | CLI session | CLI session | CLI session | Daemon + TUI + Chat | Daemon + TUI + Chat |

---

### 2. AI Provider 支援

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| Claude | ✅ | ✅ 僅此 | ❌ | ❌ | ✅ | ✅ |
| OpenAI / GPT | ✅ | ❌ | ✅ 僅此 | ❌ | ✅ | ✅ |
| Gemini | ✅ | ❌ | ❌ | ✅ 僅此 | ✅ | ✅ |
| Codex (OpenAI OAuth) | ✅ | ❌ | ✅ | ❌ | ✅ | ❌ |
| GitHub Copilot | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Nvidia NIM | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| OpenAI-compat | ✅ | ❌ | ❌ | ❌ | ✅ Ollama/LM Studio | ✅ OpenRouter 200+ |
| DeepSeek / Mistral / xAI | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
| Dispatcher 路由 | ✅ 專屬 dispatcher model | ❌ | ❌ | ❌ | ❌ | ❌ |

---

### 3. Runtime 與前端

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| TUI | ✅ bubbletea | ✅ ink | ✅ | ✅ | ✅ `openclaw tui` | ✅ React Ink |
| CLI | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| HTTP API / Web UI | ✅ gin | ❌ | ❌ | ❌ | ❌ | ✅ Web Dashboard |
| Daemon 模式 | ✅ 原生 `--daemon` | ❌ | ❌ | ❌ | ✅ systemd/launchd | ✅ gateway daemon |
| Session Canvas（HTML+SSE） | ✅ `update_page` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 具名 session | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ session picker |

---

### 4. 聊天平台整合

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| Telegram | ✅ 原生 daemon | ⚠️ Channels MCP（需 active session） | ❌ | ❌ | ✅ 原生 daemon | ✅ 原生 daemon |
| Discord | ✅ 原生 daemon | ⚠️ Channels MCP（需 active session） | ❌ | ❌ | ✅ 原生 daemon | ✅ 原生 daemon |
| iMessage | ❌ | ⚠️ Channels MCP（僅 macOS） | ❌ | ❌ | ✅ BlueBubbles | ✅ BlueBubbles |
| WhatsApp / Slack / LINE | ❌ | ❌ | ❌ | ❌ | ✅ 50+ 平台 | ✅ 20+ 平台 |
| 常駐接收（無需 session） | ✅ daemon | ❌ | ❌ | ❌ | ✅ | ✅ |
| 跨 session 發送（任一 session → chat） | ✅ `send_to_telegram_chat` / `send_to_discord_channel` | ❌ | ❌ | ❌ | ❌ | ❌ |
| OTP 驗證 | ✅ 6 碼 crypto/rand | ❌ | ❌ | ❌ | ❌ | ❌ |
| 平台原生 UI（按鈕／選單／modal） | ✅ inline keyboard / select menu / modal | ❌ | ❌ | ❌ | ⚠️ 純文字選項 | ⚠️ 純文字選項 |

> **平台層**：Agenvoy 的 Telegram 與 Discord 整合都建構在 [pardnchiu/go-bot](https://github.com/pardnchiu/go-bot) 之上，獨立維護且開源。go-bot 封裝兩個平台的 bot 協定細節 —— Agenvoy 只實作業務邏輯，平台 API 層完全交給 go-bot。

> **關鍵差異**：Claude Code Channels 需要 active session。OpenClaw 與 Hermes 雖有 daemon 但 in-chat 確認皆為純文字。Agenvoy 走平台原生 UI —— Telegram inline keyboard 與 Discord select menu / modal。另外 Agenvoy 的跨 session 發送工具讓任何 session 類型（CLI／TUI／HTTP／排程腳本）都能推訊息到 Telegram/Discord —— 無競品提供此能力。

---

### 5. Telegram 功能對比

| 功能 | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| 文字回覆 | ✅ | ✅ | ✅ | ✅ |
| 語音輸出（TTS） | ✅ Gemini TTS → OGG | ✅ ElevenLabs/Hume | ✅ Edge TTS/ElevenLabs | ❌ |
| 檔案附件輸出 | ✅ `[SEND_FILE:]` | ✅ | ✅ | ❌ |
| 接收使用者附件 | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
| 語音轉文字（STT） | ✅ Gemini，14 種格式 | ✅ Whisper/Gemini | ✅ faster-whisper（本地） | ❌ |
| Tool 確認（互動式） | ✅ 原生 inline keyboard | ⚠️ 文字 approve prompt | ⚠️ 文字選項 | ❌ |
| ask_user（picker） | ✅ 原生 button/modal | ⚠️ `/models` picker | ⚠️ 文字選項，上限 4 | ❌ |
| 排版參考（lazy-load tool） | ✅ `telegram_format` | ❌ | ❌ | ❌ |
| Scheduler 輸出推送 | ✅ | ✅ | ✅ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_telegram_chat` | ❌ | ❌ | ❌ |
| 離線接收（daemon） | ✅ | ✅ | ✅ | ❌ |

---

### 6. Discord 功能對比

| 功能 | **Agenvoy** | **OpenClaw** | **Hermes Agent** | **Claude Code Channels** |
|---------|-------------|-------------|------------------|--------------------------|
| 文字回覆 | ✅ | ✅ | ✅ | ✅ |
| 語音輸出（TTS） | ✅ Gemini TTS → OGG/OPUS | ✅ | ✅ | ❌ |
| 檔案附件輸出 | ✅ 每訊息 10 個批次 | ✅ | ✅ | ❌ |
| 接收使用者附件 | ✅ photo/doc/voice/video | ✅ | ✅ | ❌ |
| Tool 確認（互動式） | ✅ select menu button | ✅ `/model` picker | ⚠️ 文字選項 | ❌ |
| ask_user（modal） | ✅ select / multi-select / modal | ⚠️ 受限 | ⚠️ 文字選項 | ❌ |
| 排版參考（lazy-load tool） | ✅ `discord_format` | ❌ | ❌ | ❌ |
| Guild mention 守門 | ✅ | ✅ | ✅ | ❌ |
| Discord Markdown 規格遵循 | ✅ 完整規格 lazy-load tool | ⚠️ 部分 | ⚠️ 部分 | ❌ |
| 字元上限敏感 | ✅ prompt 內硬限 1600 | ❌ | ❌ | ❌ |
| 跨 session 推送（任一 session） | ✅ `send_to_discord_channel` | ❌ | ❌ | ❌ |

---

### 7. Scheduler

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| Cron 任務 | ✅ SKILL.md + cron | ✅ cloud-assisted cron/task | ❌ | ❌ | ✅ 內建 | ✅ 內建 |
| 一次性任務 | ✅ | ✅ cloud-assisted | ❌ | ❌ | ✅ `at` 格式 | ✅ 自然語言 |
| TUI CRUD | ✅ | ❌ | ❌ | ❌ | ✅ `openclaw cron` | ✅ `cronjob` tool |
| fsnotify 熱更新 | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 推送輸出到 Telegram/Discord | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ |
| AI tool 管理（add/list/remove） | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ `cronjob` tool |
| 本地執行（不需雲端） | ✅ | ❌ 依賴雲端 | ❌ | ❌ | ✅ | ✅ |

> **Scheduler 層**：Agenvoy 的 scheduler 建構在 [pardnchiu/go-scheduler](https://github.com/pardnchiu/go-scheduler)，自家維護的生態套件，提供 cron 表達式解析、一次性任務、fsnotify 熱更新、輸出回送到聊天平台。

---

### 8. Tool 生態

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| MCP 支援 | ✅ client | ✅ client | ❌ | ✅ client | ✅ client | ✅ client + server |
| 自製 tool（script-tool-add） | ✅ AI 生成 | ❌ | ❌ | ❌ | ❌ | ✅ 自動建立 skill |
| API tool 探索（search-api → add） | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Skill 系統 | ✅ SKILL.md lazy-load | ✅ CLAUDE.md | ❌ | ❌ | ✅ SKILL.md 5400+ 社群 | ✅ SKILL.md agentskills.io |
| 排版參考為 lazy-load tool | ✅ `telegram_format` / `discord_format` | ❌ | ❌ | ❌ | ❌ | ❌ |
| 圖片生成 | ✅ DALL-E/Codex Image | ❌ | ❌ | ❌ | ❌ | ❌ |
| 媒體轉錄 STT | ✅ Gemini，14 種格式 | ❌ | ❌ | ❌ | ✅ Whisper/Gemini | ✅ faster-whisper（本地） |
| TTS 語音輸出 | ✅ Gemini TTS | ❌ | ❌ | ❌ | ✅ ElevenLabs/Hume/MS | ✅ Edge TTS/ElevenLabs/OpenAI |
| Computer use / 瀏覽器 | ✅ go-rod + Playwright MCP | ✅ beta | ❌ | ❌ | ✅ Chrome CDP | ✅ Playwright（Chromium/Firefox） |

> **Tool sandbox 架構**：Agenvoy 的 Python／JavaScript／API 自製 tool 介面建構在 [pardnchiu/go-faas](https://github.com/pardnchiu/go-faas)（Function as a Service）概念之上。每個 AI 生成的 tool 都以隔離的 function unit 執行，有獨立生命週期與安全邊界。為所有受比較產品中唯一的 FaaS 等級 tool 擴展沙箱設計。

---

### 9. 記憶系統

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| 指令檔系統 | ✅ SKILL.md | ✅ CLAUDE.md | ❌ | ❌ | ✅ SKILL.md | ✅ SKILL.md |
| 對話歷史搜尋 | ✅ ToriiDB 向量搜尋 | ❌ | ❌ | ❌ | ✅ SQLite 向量 | ✅ SQLite FTS5 |
| Error memory | ✅ ToriiDB | ❌ | ❌ | ❌ | ❌ | ❌ |
| Action log | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| 長期持久記憶 | ⚠️ ToriiDB 基礎已就緒 | ⚠️ CLAUDE.md 手動 | ❌ | ❌ | ✅ Wiki 風格 MEMORY.md | ✅ MEMORY.md + USER.md |
| 跨 session 記憶 | ⚠️ 預設 session-isolated，可外掛外部記憶擴展 | ⚠️ 預設 session-isolated，可外掛外部記憶擴展 | ⚠️ 預設 session-isolated | ⚠️ 預設 session-isolated | ✅ 內建跨 session | ✅ 內建跨 session |

> **ToriiDB** 為 Agenvoy 生態自製的 embedded 向量資料庫（[pardnchiu/ToriiDB](https://github.com/pardnchiu/ToriiDB)）。不需外部服務、in-process 執行。Agenvoy 以 ToriiDB 作為記憶基礎設施，目前支撐語意對話歷史搜尋與 error memory，並作為未來長期跨 session 記憶擴展的基礎。

---

### 10. 相依與部署

| | **Agenvoy** | **Claude Code** | **Codex CLI** | **Gemini CLI** | **OpenClaw** | **Hermes Agent** |
|--|--|--|--|--|--|--|
| 直接外部相依 | **12** | 50+ | 40+ | 40+ | 龐大（pnpm monorepo） | 30–40 核心 + 60+ 選用 |
| 自家生態套件 | 5（go-bot / go-pkg / go-scheduler / ToriiDB / go-faas） | 0 | 0 | 0 | 0 | 0 |
| Runtime | Go（靜態 binary） | Node.js | Node.js | Node.js | Node.js | Python |
| 部署 | **單一 binary** | npm install | npm install | npm install | npm install | pip + docker/VPS |

---

### Agenvoy 的位置

| 面向 | 細節 |
|-----------|--------|
| **明顯優勢** | 單一 Go binary、12 個相依、自家生態（pardnchiu universe）、dispatcher model 路由、Session Canvas、平台原生 UI（真正按鈕／modal）、OTP 驗證、任一 session 跨頻道發送至 Telegram/Discord、API tool 自動探索、圖片生成、排版參考 lazy-load tool、純本地 scheduler（不需雲端） |
| **與競品持平** | Telegram/Discord daemon、TTS/STT、scheduler 輸出推送、Skill 系統、MCP、瀏覽器自動化、進站附件處理 |
| **競品領先處** | OpenClaw 50+ 平台、Hermes MCP server 模式、Hermes 本地 STT、OpenClaw/Hermes 內建跨 session 記憶、Claude Code Computer Use beta、Claude Code 雲端 cron/task |
| **Codex CLI** | 功能最少 —— 僅 CLI + TUI + OpenAI OAuth，無 daemon、無聊天平台、無 scheduler |

</details>

<details>
<summary><strong>CLI 指令</strong></summary>

> 直接以 `agen <sub>` 執行；repo Makefile 提供 `make <sub>` wrapper 供開發使用。

| 指令 | 描述 |
|---|---|
| `agen` | Attach 互動式 TUI；daemon（HTTP + Discord + Telegram + scheduler + summary cron）未跑時 fork-exec 一份。 |
| `agen cli <input>` | One-shot 跑一次 agent，每個 tool call 都會問確認。 |
| `agen run <input>` | One-shot 跑一次 agent，自動放行所有 tool。 |
| `agen stop` | 停止 daemon（SIGTERM 5s 寬限 → SIGKILL → 清 `runtime.uid`）。 |
| `agen update` | 抓最新 release、重編、停 daemon；重新 attach 載入新 binary。 |
| `agen model {add\|remove\|list\|dispatcher\|reasoning}` | 管理 provider／worker model、選 dispatcher、設 reasoning level。 |
| `agen mcp {list\|add\|remove}` | 管理 MCP server（stdio／HTTP），global 與 per-session scope。 |
| `agen session {new\|switch\|config} [name]` | 管理 CLI session；裸 `switch`／`config` 開互動 picker。 |

</details>

<details>
<summary><strong>TUI 指令</strong></summary>

> 在 `agen` 的 TUI prompt 輸入；輸入 `/` 即時過濾，popup 結束會回到 prompt。

| 指令 | 描述 |
|---|---|
| `/switch` | 切換當前 session（picker，預設高亮當前）。 |
| `/new [name]` | 建新 session；帶 name 即固定登錄至 registry。Name 會與既有 session 比對，重複則中止。 |
| `/bot` | 依序兩段 popup 編輯當前 session 的 bot：name textfield（比對其他 session，重複則中止回饋）→ description textarea（`Ctrl+S` 確認、`Enter` 換行、`Esc` 取消）。 |
| `/model [global\|session]` | Scope picker；`global` → `[add, remove]`（管理註冊表），`session` → 從已註冊 model 挑一個套到當前 session。Inline arg 跳過 scope popup。 |
| `/mcp [add\|remove]` | Action picker；`add` 走串接 popup 表單（name → transport → command/args/env 或 url/headers → scope → optional session pick），`remove` 列出 global 與 session 兩 scope 全部已設定的 server。修改後須重啟 daemon 才會載入。Inline arg 跳過 action popup。 |
| `/dispatcher` | popup 從 `cfg.Models` 挑 dispatcher model。不支援 inline arg。 |
| `/reasoning [global\|session]` | 選 `low`／`medium`／`high`，套到 dispatcher（global）或當前 session。Inline arg 跳過 scope popup。 |
| `/discord [enable\|disable]` | 切換 Discord bot 啟用／停用（token 輸入、驗證、keychain 寫入、daemon reload 全在 TUI popup chain 內完成）。Inline arg 直接切換、不彈 popup。 |
| `/telegram [enable\|disable]` | 切換 Telegram bot 啟用／停用（與 `/discord` 同模式的 in-TUI popup chain；首次與 bot 對話的 chat 必須通過 in-chat 驗證碼）。Inline arg 直接切換、不彈 popup。 |
| `/cron [add\|remove\|edit]` | 週期性排程管理。`add` 開 multiline textarea 取需求 → 派 `/scheduler-skill-creator <需求>`（缺 when/what 由 skill 透過 `ask_user` 補問）。`remove` 列出 crons → 確認 popup → `runtime.RemoveCron` + 將 skill 目錄移至 .Trash。`edit` 列出 crons → textarea 取需求 → 由 agent 自選走 `patch_cron` 或重寫 SKILL.md body。Inline arg 跳過 action popup。 |
| `/task [add\|remove\|edit]` | 一次性排程（鏡像 `/cron`；使用 `add_task` / `patch_task` / `remove_task`）。Picker 顯示 `<YYYY-MM-DD HH:MM>  <skill>`。 |
| `/sched-<name>` | 立即執行已存在的 scheduler skill body（手動 trigger）。顯示於 `/` picker 最末段（一般 skill 之後），label 套 warn-purple 標示為呼叫類。Dispatch 會加 `[執行已存在 scheduler skill: <name> · 此為手動 trigger，不是建立新 schedule]` preamble 並明示禁止 activate `scheduler-skill-creator` 或跑 init script。 |
| `/mode [cli\|web]` | 切換 `cli`（TUI 渲染）與 `web`（瀏覽器頁面）模式。Inline arg 直接切換、不彈 popup。 |
| `/update` | Popup 確認 → 走 `tea.ExecProcess` 跑 `agen stop && agen update` → 退出 TUI。 |
| `/history` | 重整顯示——清空畫面、重印 header、從當前 session 的 `action.log` 讀最近 100 筆 entry 重新渲染。 |
| `/log` | 以 `$PAGER`（fallback `less -Rf +G`，直接跳到檔尾）開啟 raw `action.log`。`\x1F` marker 會還原為實際換行以利閱讀。 |
| `/clear` | 僅清除當前視窗顯示，等同 terminal `clear`；對話記憶不動。 |
| `/exit`, `/quit` | 退出 TUI（daemon 仍在跑，重 `agen` 即可 attach）。 |

</details>

<details>
<summary><strong>內建工具</strong></summary>

> Tool 以 stub 形式 lazy load，首次呼叫才展開完整 schema。參數與分派細節見 [Tools wiki](https://github.com/pardnchiu/agenvoy/wiki/工具系統)。

| Tool | 描述 |
|---|---|
| **檔案** |  |
| `read_file` | 讀取 text／PDF／DOCX／PPTX／CSV／TSV／image。 |
| `write_file` | 寫入檔案，已存在則覆蓋。 |
| `patch_file` | 在檔案內以 exact match 替換字串。 |
| `list_files` | 列出目錄項目；`recursive=true` 走子樹檔案。 |
| `glob_files` | 以 glob pattern 在目錄中尋找檔案。 |
| `search_files` | 以 RE2 regex 搜尋目錄內檔案內容。 |
| **網頁** |  |
| `fetch_page` | 抓取網頁並回傳 Markdown。 |
| `save_page_to_file` | 抓取網頁並存成本地檔案。 |
| `search_web` | 走 DuckDuckGo Lite 搜尋，回前 10 筆結果。 |
| `fetch_google_rss` | 搜尋 Google News RSS，回標題／摘要／連結。 |
| `fetch_yahoo_finance` | 查 Yahoo Finance 報價與 K 線（OHLCV）。 |
| `fetch_youtube_transcript` | 抓 YouTube 影片逐字稿含時間戳。*(gemini needed)* |
| `transcribe_media` | 將本地音訊／影片檔（ogg、mp3、wav、m4a、flac、aac、mp4、mov、webm、mpeg、3gp 等）轉成逐字稿，單檔上限 20 MiB。*(gemini needed)* |
| `send_http_request` | 對指定 URL 發 HTTP 請求。 |
| **Shell** |  |
| `run_command` | 以 argv 執行 binary，回 stdout/stderr 合併輸出。 |
| **渲染** |  |
| `update_page` | 覆寫當前 session 的 HTML 頁面，瀏覽器分頁自動 reload。 |
| `generate_image` | 透過 gpt-image-2 生圖（尺寸與品質由 user 互動選擇）。*(codex needed)* |
| **頻道** |  |
| `list_telegram_chat` | 列出已授權的 Telegram chat（id + name）。*(telegram needed)* |
| `send_to_telegram_chat` | 以 chat_id 將 HTML 格式訊息送到已授權的 Telegram chat。*(telegram needed)* |
| `telegram_format` | 回傳 Telegram HTML 排版參考（允許 tag、escape 規則、檔案／語音 marker）。*(telegram needed)* |
| `list_discord_channel` | 列出已授權的 Discord channel（id + name）。*(discord needed)* |
| `send_to_discord_channel` | 以 channel_id 將 markdown 格式訊息送到已授權的 Discord channel。*(discord needed)* |
| `discord_format` | 回傳 Discord markdown 排版參考（允許 markdown、特殊 token、檔案／語音 marker）。*(discord needed)* |
| **計算** |  |
| `calculate` | 計算數學表達式，回精確結果。 |
| **探索** |  |
| `list_tools` | 列出當前所有 built-in 與動態載入的 tool。 |
| `search_tools` | 以 keyword 搜 tool 並把匹配項注入當前 request。 |
| `activate_skill` | 以名稱拉取 skill 的參考內容。 |
| **互動** |  |
| `ask_user` | 對使用者問一或多個問題並回答案。 |
| `store_secret` | 以遮罩輸入向使用者要 secret 並存進系統 keychain。 |
| **記憶** |  |
| `search_conversation_history` | 在本 session 歷史以 keyword + semantic 並聯搜尋。 |
| `search_error_memory` | 語意搜尋過去 tool error 記錄，命中即續期 3 個月 TTL。 |
| `read_error_memory` | 以 hash 拉取單筆過去 tool error 內容。 |
| `remember_error` | 寫入一筆 tool error 記錄供未來查詢。 |
| **Agent** |  |
| `invoke_subagent` | 在內部 subagent session 跑子任務，回最終文字。 |
| `invoke_external_agent` | 喚起單一外部 CLI（codex／copilot／claude／gemini）取得第二意見。 |
| `cross_review_with_external_agents` | 把已完成結果並聯丟給所有可用外部 CLI 互審。 |
| `review_result` | 對結果與原任務做比對，回具體問題與改進建議。 |
| **Scheduler** |  |
| `add_task` | 把既有 scheduler skill 綁定在特定時間執行一次（`+5m`／`HH:MM`／`YYYY-MM-DD HH:MM`／RFC3339）。 |
| `add_cron` | 把既有 scheduler skill 綁定於 5 欄 cron expression 週期觸發。 |
| `patch_task` / `patch_cron` | 依 skill name 改既有 task／cron 的時間（只動時間、不動 skill body）。 |
| `remove_task` / `remove_cron` | 依 skill name 取消 task／cron；綁定的 scheduler skill 目錄一併搬到 `.Trash/`。 |
| **Skill Git** |  |
| `skill_git_commit` / `skill_git_log` / `skill_git_rollback` | Commit／列出／回滾 `~/.config/agenvoy/skills` 的 git 歷史。 |

動態 tool 群（自動註冊、上表不列）：MCP server 注入的 `mcp__<server>__<tool>`、`extensions/apis/*.json` 註冊的 `api_<name>`、`extensions/scripts/<name>/` 註冊的 `script_<name>`。

</details>

## Wiki

| English | 中文 |
|---|---|
| [Getting Started](https://github.com/pardnchiu/agenvoy/wiki/Getting-Started) | [新手入門](https://github.com/pardnchiu/agenvoy/wiki/新手入門) |
| [Architecture](https://github.com/pardnchiu/agenvoy/wiki/Architecture) | [架構](https://github.com/pardnchiu/agenvoy/wiki/架構) |
| [Core Concepts](https://github.com/pardnchiu/agenvoy/wiki/Core-Concepts) | [核心概念](https://github.com/pardnchiu/agenvoy/wiki/核心概念) |
| [Providers](https://github.com/pardnchiu/agenvoy/wiki/Providers) | [Provider 設定](https://github.com/pardnchiu/agenvoy/wiki/Provider-設定) |
| [Tools](https://github.com/pardnchiu/agenvoy/wiki/Tools) | [工具系統](https://github.com/pardnchiu/agenvoy/wiki/工具系統) |
| [Memory System](https://github.com/pardnchiu/agenvoy/wiki/Memory-System) | [記憶系統](https://github.com/pardnchiu/agenvoy/wiki/記憶系統) |
| [Skill System](https://github.com/pardnchiu/agenvoy/wiki/Skill-System) | [Skill 系統](https://github.com/pardnchiu/agenvoy/wiki/Skill-系統) |
| [MCP Integration](https://github.com/pardnchiu/agenvoy/wiki/MCP-Integration) | [MCP 整合](https://github.com/pardnchiu/agenvoy/wiki/MCP-整合) |
| [Security and Sandbox](https://github.com/pardnchiu/agenvoy/wiki/Security-and-Sandbox) | [安全與沙箱](https://github.com/pardnchiu/agenvoy/wiki/安全與沙箱) |
| [CLI Reference](https://github.com/pardnchiu/agenvoy/wiki/CLI-Reference) | [命令列參考](https://github.com/pardnchiu/agenvoy/wiki/命令列參考) |
| [Configuration](https://github.com/pardnchiu/agenvoy/wiki/Configuration) | [設定檔](https://github.com/pardnchiu/agenvoy/wiki/設定檔) |

## 授權

本專案採用 [Apache License 2.0](../LICENSE)。

## 貢獻者

想丟想法 [開個 issue](https://github.com/pardnchiu/agenvoy/issues/new) 聊聊也行。

<a href="https://github.com/pardnchiu/agenvoy/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=pardnchiu/agenvoy&cache_bust=2026-05-12" alt="Agenvoy 貢獻者" />
</a>

## Star History

<a href="https://star-history.com/#pardnchiu/agenvoy&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&theme=dark&cache_bust=2026-05-12" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
    <img alt="Agenvoy star history" src="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
  </picture>
</a>

曲線往上走 —— 那就是我們想看到的訊號。點 ★ 推它一把。

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
