# 新手入門

> [English](https://github.com/agenvoy/Agenvoy/wiki/Getting-Started)

## 前置需求

- Go 1.25.1 以上
- Linux（bubblewrap 沙箱；缺 `bwrap` 會嘗試 apt/dnf/yum/pacman/apk 自動安裝）或 macOS（`sandbox-exec`）
- 至少一個 LLM provider 帳號：Copilot 訂閱、或 OpenAI／Claude／Gemini／Nvidia API key 任一
- 選用：`pdftotext`（poppler-utils）—— `read_file` 解析 PDF 需要
- 選用：`OPENAI_API_KEY` —— 啟用語意搜尋（`text-embedding-3-small`）

## 安裝

```bash
git clone https://github.com/pardnchiu/agenvoy.git
cd agenvoy
make build
```

`make build` 會編譯、把最新 git tag 嵌入為 `projectVersion`、安裝 binary 至 `/usr/local/bin/agen`。

## 設定至少一個 Provider

Agenvoy 需要至少一個 LLM provider 才能運作：

```bash
agen model add
```

互動式 prompt 引導選 provider、選 model、儲存憑證。Token 寫入 OS keychain（macOS `security`、Linux `secret-tool`、加密檔 fallback），service 名稱固定 `agenvoy`。

主設定檔位於 `~/.config/agenvoy/config.json`。

## 第一次執行

```bash
# 建立具名 cli- session 並切為主指標
agen session new my-assistant

# 啟動完整堆疊（TUI + Discord + Telegram + REST）
make app
```

TUI 啟動後按 **`i`** 開啟 Message 輸入欄、按 **Enter** 送出（`Shift+Enter` 在支援 modifier 的終端可插入換行）。按 **`c`** 開啟 Command（`$`）輸入欄。**`Tab`** 切換主畫面 Content / Logs；**`Ctrl+P`** 開啟 co-work dashboard（Sessions / Log / Pending 三 panel）。

單次 CLI 用法：

```bash
make cli "幫我看一下 main.go 的最新變動"
make run "用 playwright 打開 example.com 截圖"
```

`make cli` 對非 read-only tool 每次都會 confirm；`make run` 全部自動放行。

## 下一步

- [核心概念](https://github.com/agenvoy/Agenvoy/wiki/核心概念) —— session、agent routing、iteration loop、三段式 tool dispatch
- [Provider 設定](https://github.com/agenvoy/Agenvoy/wiki/Provider-設定) —— 支援的 LLM 後端與 planner model
- [MCP 整合](https://github.com/agenvoy/Agenvoy/wiki/MCP-整合) —— 接入外部工具 server
- [命令列參考](https://github.com/agenvoy/Agenvoy/wiki/命令列參考) —— 完整指令清單
