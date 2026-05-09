# Providers

> [中文](https://github.com/agenvoy/Agenvoy/wiki/Provider-設定)

Agenvoy supports seven LLM providers behind a unified `Agent.Send()` interface.

## Supported list

| Provider | Config name | Notes |
|---|---|---|
| Anthropic Claude | `claude` | Messages API; parallel tool use enabled by default |
| OpenAI | `openai` | Chat Completions / Responses API |
| OpenAI Codex | `codex` | OAuth login (uses your ChatGPT / Codex account, no API key); SSE streaming; auto prompt-cache key (`sha256(instructions)`) |
| Google Gemini | `gemini` | gemini-2.x / 3.x families |
| GitHub Copilot | `copilot` | Requires GitHub OAuth (one-shot login flow) |
| Nvidia NIM | `nvidia` | Llama, Mistral, and other open-weight hosted models |
| Compat | `compat` | Any custom OpenAI-compatible endpoint |

> **On the `providors/` spelling** — intentional convention; do not "fix" it. Provider JSON catalogs live under `configs/jsons/providors/`. Six static files ship today (`claude.json`, `openai.json`, `codex.json`, `gemini.json`, `copilot.json`, `nvidia.json`); `compat` is constructed at runtime from user-supplied endpoints.

## Provider configuration

```bash
agen model add          # Interactive provider/model add
agen model remove       # Interactive provider/model remove
agen model list         # List registered models
agen model planner      # Choose the planner model
agen model reasoning    # Set planner reasoning effort: low / medium / high / xhigh
```

Credentials (API keys, OAuth tokens) are stored in the OS keychain under service `agenvoy`, never in plain JSON.

## Planner model

The planner LLM decides which worker model handles each task. It is invoked through `SelectAgent()` before `Execute()` enters its iteration loop, receiving the user input plus a hint about any matched skill.

Configure via `agen model planner` (model selection) and `agen model reasoning` (reasoning effort).

## Streaming

Only `openaiCodex` uses SSE for response streaming (`parseSSEStream` accumulates `argsBuf` per `item_id`). Other providers receive the full response in one shot per turn.

## Parallel tool calls

- **Claude Messages API** — parallel tool use is on by default
- **OpenAI Responses API** — `parallel_tool_calls=true` left on
- The agenvoy execution engine still serializes commit (Pass 3) and respects per-tool concurrency markers

## Prompt caching

`openaiCodex/send.go` computes `sha256(instructions)` and sends it as `prompt_cache_key`. Anthropic and OpenAI both honor automatic prefix caching at ≥1024 tokens, so no explicit cache markers are needed.

## Adding a custom OpenAI-compatible endpoint

Use the `compat` provider type and point at any endpoint that accepts the OpenAI Chat Completions schema:

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

Run `agen model add` and pick the `compat` type to walk through the interactive setup; new models become available on next agent invocation.
