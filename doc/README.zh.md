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
Go-native dispatcher · Planner 將每個步驟派給最適合的模型 · Subagent 在同一 process 內協作
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

## CLI 指令

> 直接以 `agen <sub>` 執行；repo Makefile 提供 `make <sub>` wrapper 供開發使用。

| 指令 | 描述 |
|---|---|
| `agen` | Attach 互動式 TUI；daemon（HTTP + Discord + Telegram + scheduler + summary cron）未跑時 fork-exec 一份。 |
| `agen cli <input>` | One-shot 跑一次 agent，每個 tool call 都會問確認。 |
| `agen run <input>` | One-shot 跑一次 agent，自動放行所有 tool。 |
| `agen stop` | 停止 daemon（SIGTERM 5s 寬限 → SIGKILL → 清 `runtime.uid`）。 |
| `agen update` | 抓最新 release、重編、停 daemon；重新 attach 載入新 binary。 |
| `agen model {add\|remove\|list\|planner\|reasoning}` | 管理 provider／worker model、選 planner、設 reasoning level。 |
| `agen mcp {list\|add\|remove}` | 管理 MCP server（stdio／HTTP），global 與 per-session scope。 |
| `agen session {new\|switch\|config} [name]` | 管理 CLI session；裸 `switch`／`config` 開互動 picker。 |

## TUI 指令

> 在 `agen` 的 TUI prompt 輸入；輸入 `/` 即時過濾，popup 結束會回到 prompt。

| 指令 | 描述 |
|---|---|
| `/switch` | 切換當前 session（picker，預設高亮當前）。 |
| `/new [name]` | 建新 session；帶 name 即固定登錄至 registry。Name 會與既有 session 比對，重複則中止。 |
| `/bot` | 依序兩段 popup 編輯當前 session 的 bot：name textfield（比對其他 session，重複則中止回饋）→ description textarea（`Ctrl+S` 確認、`Enter` 換行、`Esc` 取消）。 |
| `/model [global\|session]` | Scope picker；`global` → `[add, remove]`（管理註冊表），`session` → 從已註冊 model 挑一個套到當前 session。Inline arg 跳過 scope popup。 |
| `/mcp [add\|remove]` | Action picker；`add` 走串接 popup 表單（name → transport → command/args/env 或 url/headers → scope → optional session pick），`remove` 列出 global 與 session 兩 scope 全部已設定的 server。修改後須重啟 daemon 才會載入。Inline arg 跳過 action popup。 |
| `/planner` | popup 從 `cfg.Models` 挑 planner model。不支援 inline arg。 |
| `/reasoning [global\|session]` | 選 `low`／`medium`／`high`，套到 planner（global）或當前 session。Inline arg 跳過 scope popup。 |
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

## 內建工具

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
| `fetch_youtube_transcript` | 抓 YouTube 影片逐字稿含時間戳。 |
| `send_http_request` | 對指定 URL 發 HTTP 請求。 |
| **Shell** |  |
| `run_command` | 以 argv 執行 binary，回 stdout/stderr 合併輸出。 |
| **渲染** |  |
| `update_page` | 覆寫當前 session 的 HTML 頁面，瀏覽器分頁自動 reload。 |
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
