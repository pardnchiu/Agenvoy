# REST API

Started by `make app`. The HTTP server binds to `127.0.0.1` only — LAN clients cannot reach the daemon. CORS middleware with an origin whitelist gates cross-origin access (required for the `web.agenvoy.com` co-work dashboard).

## Endpoints

| Endpoint | Description |
|---|---|
| `POST /v1/chat/completions` | OpenAI-compatible chat completions (stateless) |
| `POST /v1/send` | Send a message; body `{sid?, persist?, text}` |
| `GET /v1/sessions` | List all sessions with status |
| `GET /v1/session/:sid/status` | Read `status.json` (404 if session missing) |
| `GET /v1/session/:sid/log` | SSE stream of `action.log` (1 s ticker, `: ping` heartbeat) |
| `GET /v1/log?sessions=a,b,c` | Multiplexed SSE — single connection streams events from multiple sessions, each event tagged with `session` field |
| `GET /v1/session/:sid/pending` | List pending confirm/ask tasks for a session |
| `GET /v1/session/:sid/pending/:hash/questions` | Get questions for a specific pending task |
| `POST /v1/session/:sid/pending/:hash/resume` | Submit answers to resume a pending task |
| `POST /v1/session/:sid/event` | Publish a session event (localhost only) |
| `GET /v1/tools` | List registered tools |
| `POST /v1/tool/:tool_name` | Invoke a tool directly |
| `GET /v1/key` | Read a value from keychain (localhost only) |
| `POST /v1/key` | Write a value to keychain |

## `POST /v1/send` semantics

| `persist` | `sid` | Result |
|---|---|---|
| `false` (default) | empty | Creates `temp-<uuid>`, reaped after 30 min idle |
| `true` | empty | Creates `http-<uuid>`, retained permanently |
| any | provided | Uses the supplied sid (`persist` is ignored) |
