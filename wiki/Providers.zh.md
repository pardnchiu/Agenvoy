# Provider 設定

> [English](Providers.md)

Agenvoy 透過統一的 `Agent.Send()` 介面支援 9 家 LLM provider。

## 支援清單

| Provider | 設定名稱 | 備註 |
|---|---|---|
| Anthropic Claude | `claude` | Messages API；預設啟用 parallel tool use |
| OpenAI | `openai` | Chat Completions / Responses API |
| OpenAI Codex | `codex` | OAuth 登入（用你的 ChatGPT／Codex 帳號，不需 API key）；SSE 串流；自動 prompt-cache key（`sha256(instructions)`） |
| Google Gemini | `gemini` | gemini-2.x / 3.x 系列 |
| GitHub Copilot | `copilot` | 需 GitHub OAuth（一次性登入） |
| Nvidia NIM | `nvidia` | Llama、Mistral 等 hosted 開源模型 |
| xAI Grok | `grok` | grok-4／grok-3 系列含 `grok-code-fast-1`；非串流 HTTP client |
| DeepSeek | `deepseek` | `deepseek-chat`（tool use）與 `deepseek-reasoner`（CoT，停用 temperature）；非串流 HTTP client |
| Compat | `compat` | 任何 OpenAI 相容格式的自訂 endpoint |

> **`providors/` 拼寫** —— 是慣例非錯字，請勿「修正」。Provider JSON 目錄在 `configs/jsons/providors/`，現有 8 份靜態 catalog（`claude.json`、`openai.json`、`codex.json`、`gemini.json`、`copilot.json`、`nvidia.json`、`grok.json`、`deepseek.json`）；`compat` 由使用者輸入動態建構。

## Provider 配置

```bash
agen model add          # 互動式新增 provider／model
agen model remove       # 互動式移除 provider／model
agen model list         # 列出已註冊 model
agen model dispatcher   # 選 dispatcher model
agen model reasoning    # 設 dispatcher 推理層級：low / medium / high / xhigh
```

憑證（API key、OAuth token）放 OS keychain（service `agenvoy`），絕不寫進 JSON。

## Dispatcher Model

Dispatcher LLM 決定每個任務派分給哪個 worker model。`Execute()` 進入迭代迴圈前由 `SelectAgent()` 呼叫 dispatcher，輸入是 user message + skill 命中提示。

設定：`agen model dispatcher`（選 model）與 `agen model reasoning`（reasoning effort）。

## 串流

只有 `openaiCodex` 走 SSE 串流（`parseSSEStream` 依 `item_id` 累積 `argsBuf`）。其他 provider 每輪都是一次拿完整 response。

## Parallel Tool Calls

- **Claude Messages API** —— parallel tool use 預設開
- **OpenAI Responses API** —— `parallel_tool_calls=true` 維持預設
- agenvoy 執行引擎仍序列化 commit（Pass 3）並遵守 per-tool concurrency 標記

## Prompt Caching

`openaiCodex/send.go` 對 `instructions` 算 `sha256` 當 `prompt_cache_key`。Anthropic 與 OpenAI 都自動對 ≥1024 token 的 prefix 快取，不需顯式 cache marker。

## 自訂 OpenAI 相容 Endpoint

用 `compat` provider 類型，指向任何吃 OpenAI Chat Completions schema 的 endpoint。URL 慣例對齊 Zed：**填到 `/v1` 為止**（例：`http://192.168.1.10:4000/v1`，Ollama 預設 `http://localhost:11434/v1`）。`compat/send.go` 只 append `/chat/completions`。

```
/providor → name: VLLM
            URL:  http://192.168.1.10:4000/v1
            API key: <bearer token，或留空>
            Model: gemma3-27b-it          （成為 compat[VLLM]@gemma3-27b-it）
```

### Storage 分軌（URL vs key）

| 內容 | 位置 | API |
|---|---|---|
| URL | `~/.config/agenvoy/config.json` `compats[].URL` | `session.UpsertCompat` / `session.GetCompatURL` |
| API key | OS keychain | `keychain.Set("COMPAT_<NAME>_API_KEY", value)` |

`compat.New` 透過 `session.GetCompatURL(instanceName)` 讀 URL。`COMPAT_<NAME>_URL` keychain key 已下線（刻意移除）。

### 已測 compat 目標

| 目標 | 通過 | 備註 |
|---|---|---|
| Ollama | ✅ | 預設 `http://localhost:11434/v1` |
| LM Studio | ✅ | |
| vLLM | ✅ | tool use 需 server 啟動 `--enable-auto-tool-choice --tool-call-parser <name>` |
| llama.cpp server | ✅ | |
| LiteLLM proxy | ✅ | virtual key 當 Bearer token |
| Groq / Together / DeepInfra / OpenRouter / Fireworks | ✅ | |
| Azure OpenAI | ❌ | 需 `api-key` header（非 `Bearer`）+ `?api-version=` query —— 不支援 |
| Reasoning-only models（o1、deepseek-r1、QwQ） | ⚠️ | compat hardcoded `temperature: 0.2`；部分 server 會 422 |

## Send timeout（3 層）

Send 端 timeout 有三層獨立的層級，各自捕捉不同失敗模式：

| 層 | 值 | 捕捉 | 位置 |
|---|---|---|---|
| **Transport** `ResponseHeaderTimeout` | `10s` | Backend 卡在 headers 階段（健康 SSE 應 <1s 回 headers；高負載 ≤ 5s；10s = 10× margin） | `provider.NewHTTPClient()`（雲端非 SSE）+ `openaiCodex/new.go::newHTTPClient()`（SSE） |
| **`http.Client.Timeout`** | `5m` 非 SSE / `10m` SSE | 完整請求（headers + body） | per-provider client |
| **`execute.go::AgentSendTimeout`** | env `AGENT_SEND_TIMEOUT_SECONDS`，default `600s` | Exec 層 ceiling，用 `context.WithTimeout` 包 ctx | `internal/agents/exec/execute.go` |

對非 SSE provider，`Client.Timeout=5m` 永遠先 fire（exec wrap 是 10m）。Exec wrap 主要為 codex SSE（10m client）與長 reasoning model 提供統一上限。

### HTTP client factory 分軌

| Provider 類別 | Factory | 設定 |
|---|---|---|
| 雲端非 SSE（claude / copilot / gemini / nvidia / openai） | `provider.NewHTTPClient()` | `Timeout=5m` + `ResponseHeaderTimeout=10s` |
| 雲端 SSE（openaiCodex） | `openaiCodex/new.go::newHTTPClient()` | `Timeout=10m` + `ResponseHeaderTimeout=10s` |
| 本地 / 自架（compat） | inline `&http.Client{Timeout: 5 * time.Minute}` | **無** `ResponseHeaderTimeout` —— Ollama／vLLM／llama.cpp 冷啟動可能 hold 30-90s 才回 headers，10s 必 100% 誤殺 |

本地 compat **不**走 factory 是設計。自架 backend 的冷啟動容忍是不可妥協的。

### Retry 語意

- `sendFailCount` 對 timeout/network error **無條件累計**（payload 沒到 model，sig 比較無意義）
- 對 content-level error（parse 失敗、4xx 同 body、garbage response）走 sig-based：同 sig → 累計，不同 → reset
- `sendFailCount >= MaxRetry`（default 3）→ MaxRetry 耗盡路徑 emit `sendText` + `EventDone`，message 依分支（timeout / context-length / generic）
- 重試中（`sendFailCount < MaxRetry`）→ **只** `slog.Warn`；不送 chat event（避免 "retrying 1/3、2/3" 雜訊 —— 只有最終結果到 user）

OAuth device-code polling（`copilot/login.go`）有獨立 `http.Client{Timeout: 30s}` per-poll —— 零 timeout 會讓 GitHub OAuth backend hang 鎖死整個登入流程。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。
