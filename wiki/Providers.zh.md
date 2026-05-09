# Provider 設定

> [English](https://github.com/agenvoy/Agenvoy/wiki/Providers)

Agenvoy 透過統一的 `Agent.Send()` 介面支援 7 家 LLM provider。

## 支援清單

| Provider | 設定名稱 | 備註 |
|---|---|---|
| Anthropic Claude | `claude` | Messages API；預設啟用 parallel tool use |
| OpenAI | `openai` | Chat Completions / Responses API |
| OpenAI Codex | `codex` | OAuth 登入（用你的 ChatGPT／Codex 帳號，不需 API key）；SSE 串流；自動 prompt-cache key（`sha256(instructions)`） |
| Google Gemini | `gemini` | gemini-2.x / 3.x 系列 |
| GitHub Copilot | `copilot` | 需 GitHub OAuth（一次性登入） |
| Nvidia NIM | `nvidia` | Llama、Mistral 等 hosted 開源模型 |
| Compat | `compat` | 任何 OpenAI 相容格式的自訂 endpoint |

> **`providors/` 拼寫** —— 是慣例非錯字，請勿「修正」。Provider JSON 目錄在 `configs/jsons/providors/`，現有 6 份靜態 catalog（`claude.json`、`openai.json`、`codex.json`、`gemini.json`、`copilot.json`、`nvidia.json`）；`compat` 由使用者輸入動態建構。

## Provider 配置

```bash
agen model add          # 互動式新增 provider／model
agen model remove       # 互動式移除 provider／model
agen model list         # 列出已註冊 model
agen model planner      # 選 planner model
agen model reasoning    # 設 planner 推理層級：low / medium / high / xhigh
```

憑證（API key、OAuth token）放 OS keychain（service `agenvoy`），絕不寫進 JSON。

## Planner Model

Planner LLM 決定每個任務派分給哪個 worker model。`Execute()` 進入迭代迴圈前由 `SelectAgent()` 呼叫 planner，輸入是 user message + skill 命中提示。

設定：`agen model planner`（選 model）與 `agen model reasoning`（reasoning effort）。

## 串流

只有 `openaiCodex` 走 SSE 串流（`parseSSEStream` 依 `item_id` 累積 `argsBuf`）。其他 provider 每輪都是一次拿完整 response。

## Parallel Tool Calls

- **Claude Messages API** —— parallel tool use 預設開
- **OpenAI Responses API** —— `parallel_tool_calls=true` 維持預設
- agenvoy 執行引擎仍序列化 commit（Pass 3）並遵守 per-tool concurrency 標記

## Prompt Caching

`openaiCodex/send.go` 對 `instructions` 算 `sha256` 當 `prompt_cache_key`。Anthropic 與 OpenAI 都自動對 ≥1024 token 的 prefix 快取，不需顯式 cache marker。

## 自訂 OpenAI 相容 Endpoint

用 `compat` provider 類型，指向任何吃 OpenAI Chat Completions schema 的 endpoint：

```json
{
  "type": "compat",
  "name": "my-local",
  "base_url": "http://localhost:8080/v1",
  "models": [
    {"id": "qwen2.5-coder-32b"}
  ]
}
```

跑 `agen model add` 選 `compat` 型別走互動式設定；新 model 在下次 agent 呼叫時可用。
