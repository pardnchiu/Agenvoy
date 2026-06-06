# Providers

> [中文](Providers.zh.md)

Agenvoy supports nine LLM providers behind a unified `Agent.Send()` interface.

## Supported list

| Provider | Config name | Notes |
|---|---|---|
| Anthropic Claude | `claude` | Messages API; parallel tool use enabled by default |
| OpenAI | `openai` | Chat Completions / Responses API |
| OpenAI Codex | `codex` | OAuth login (uses your ChatGPT / Codex account, no API key); SSE streaming; auto prompt-cache key (`sha256(instructions)`) |
| Google Gemini | `gemini` | gemini-2.x / 3.x families |
| GitHub Copilot | `copilot` | Requires GitHub OAuth (one-shot login flow) |
| Nvidia NIM | `nvidia` | Llama, Mistral, and other open-weight hosted models |
| xAI Grok | `grok` | grok-4 / grok-3 families incl. `grok-code-fast-1`; non-streaming HTTP client |
| DeepSeek | `deepseek` | `deepseek-chat` (tool use) and `deepseek-reasoner` (CoT, temperature disabled); non-streaming HTTP client |
| Compat | `compat` | Any custom OpenAI-compatible endpoint |

> **On the `providors/` spelling** — intentional convention; do not "fix" it. Provider JSON catalogs live under `configs/jsons/providors/`. Eight static files ship today (`claude.json`, `openai.json`, `codex.json`, `gemini.json`, `copilot.json`, `nvidia.json`, `grok.json`, `deepseek.json`); `compat` is constructed at runtime from user-supplied endpoints.

## Provider configuration

```bash
agen model add          # Interactive provider/model add
agen model remove       # Interactive provider/model remove
agen model list         # List registered models
agen model dispatcher   # Choose the dispatcher model
agen model reasoning    # Set dispatcher reasoning effort: low / medium / high / xhigh
```

Credentials (API keys, OAuth tokens) are stored in the OS keychain under service `agenvoy`, never in plain JSON.

## Dispatcher model

The dispatcher LLM decides which worker model handles each task. It is invoked through `SelectAgent()` before `Execute()` enters its iteration loop, receiving the user input plus a hint about any matched skill.

Configure via `agen model dispatcher` (model selection) and `agen model reasoning` (reasoning effort).

## Streaming

Only `openaiCodex` uses SSE for response streaming (`parseSSEStream` accumulates `argsBuf` per `item_id`). Other providers receive the full response in one shot per turn.

## Parallel tool calls

- **Claude Messages API** — parallel tool use is on by default
- **OpenAI Responses API** — `parallel_tool_calls=true` left on
- The agenvoy execution engine still serializes commit (Pass 3) and respects per-tool concurrency markers

## Prompt caching

`openaiCodex/send.go` computes `sha256(instructions)` and sends it as `prompt_cache_key`. Anthropic and OpenAI both honor automatic prefix caching at ≥1024 tokens, so no explicit cache markers are needed.

## Adding a custom OpenAI-compatible endpoint

Use the `compat` provider type and point at any endpoint that accepts the OpenAI Chat Completions schema. URL convention follows Zed: **enter the URL up to `/v1`** (e.g. `http://192.168.1.10:4000/v1`, Ollama default `http://localhost:11434/v1`). `compat/send.go` appends only `/chat/completions`.

```
/providor → name: VLLM
            URL:  http://192.168.1.10:4000/v1
            API key: <bearer token, or blank>
            Model: gemma3-27b-it          (becomes compat[VLLM]@gemma3-27b-it)
```

### Storage split (URL vs key)

| What | Where | API |
|---|---|---|
| URL | `~/.config/agenvoy/config.json` `compats[].URL` | `session.UpsertCompat` / `session.GetCompatURL` |
| API key | OS keychain | `keychain.Set("COMPAT_<NAME>_API_KEY", value)` |

`compat.New` reads URL via `session.GetCompatURL(instanceName)`. There is no `COMPAT_<NAME>_URL` keychain key (intentionally removed).

### Tested compat targets

| Target | Works | Notes |
|---|---|---|
| Ollama | ✅ | default `http://localhost:11434/v1` |
| LM Studio | ✅ | |
| vLLM | ✅ | `--enable-auto-tool-choice --tool-call-parser <name>` for tool use |
| llama.cpp server | ✅ | |
| LiteLLM proxy | ✅ | virtual key as Bearer token |
| Groq / Together / DeepInfra / OpenRouter / Fireworks | ✅ | |
| Azure OpenAI | ❌ | needs `api-key` header (not `Bearer`) + `?api-version=` query — not supported |
| Reasoning-only models (o1, deepseek-r1, QwQ) | ⚠️ | compat sends `temperature: 0.2` hardcoded; some servers 422 |

## Send timeout (3 layers)

Send-side timeout has three independent layers, each catching a different failure mode:

| Layer | Value | Catches | Where |
|---|---|---|---|
| **Transport** `ResponseHeaderTimeout` | `10s` | Backend stuck before returning headers (healthy SSE returns <1s; high load ≤ 5s; 10s = 10× margin) | `provider.NewHTTPClient()` (cloud non-SSE) + `openaiCodex/new.go::newHTTPClient()` (SSE) |
| **`http.Client.Timeout`** | `5m` non-SSE / `10m` SSE | Full request (headers + body) | per-provider client |
| **`execute.go::AgentSendTimeout`** | env `AGENT_SEND_TIMEOUT_SECONDS`, default `600s` | Exec-layer ceiling via `context.WithTimeout` | `internal/agents/exec/execute.go` |

For non-SSE providers, `Client.Timeout=5m` always fires before the exec wrap (which is 10m). The exec wrap exists primarily for codex SSE (10m client) and long-reasoning models.

### HTTP client factory split

| Provider category | Factory | Config |
|---|---|---|
| Cloud non-SSE (claude / copilot / gemini / nvidia / openai) | `provider.NewHTTPClient()` | `Timeout=5m` + `ResponseHeaderTimeout=10s` |
| Cloud SSE (openaiCodex) | `openaiCodex/new.go::newHTTPClient()` | `Timeout=10m` + `ResponseHeaderTimeout=10s` |
| Local / self-hosted (compat) | inline `&http.Client{Timeout: 5 * time.Minute}` | **no** `ResponseHeaderTimeout` — Ollama / vLLM / llama.cpp cold-start may hold 30-90s before headers; 10s would 100% false-positive |

Local compat is **not** routed through the factory by design. Cold-start tolerance is non-negotiable for self-hosted backends.

### Retry semantics

- `sendFailCount` accumulates **unconditionally** for timeout/network errors (payload didn't reach the model; signature comparison is meaningless)
- For content-level errors (parse failure, 4xx with body, garbage response), retry is sig-based — same payload signature → counter increments; different → reset
- `sendFailCount >= MaxRetry` (default 3) → MaxRetry-exhausted path emits `sendText` + `EventDone` with a branch-specific message (timeout / context-length / generic)
- During retries (`sendFailCount < MaxRetry`) → **only** `slog.Warn` is emitted; no chat event surfaces (avoids noisy "retrying 1/3, 2/3" spam — only the final outcome reaches the user)

OAuth device-code polling (`copilot/login.go`) uses a separate `http.Client{Timeout: 30s}` per poll — zero timeout would let GitHub OAuth backend hang and lock the entire login flow.

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
