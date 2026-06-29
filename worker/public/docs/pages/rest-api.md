# REST API

Started by `make app` (default port `:3000`).

| Endpoint | Description |
|---|---|
| `POST /v1/send` | Send a message; body `{sid?, persist?, text}` |
| `POST /v1/key` | Write a value to keychain |
| `GET /v1/key` | Read a value from keychain |
| `GET /v1/tools` | List registered tools |
| `POST /v1/tool/:tool_name` | Invoke a tool directly |
| `GET /v1/session/:sid/status` | Read `status.json` (404 if session missing) |
| `GET /v1/session/:sid/log` | SSE stream of `action.log` (1 s ticker, `: ping` every 15 idle ticks) |

## `POST /v1/send` semantics

| `persist` | `sid` | Result |
|---|---|---|
| `false` (default) | empty | Creates `temp-<uuid>`, reaped after 1 h idle |
| `true` | empty | Creates `http-<uuid>`, retained permanently |
| any | provided | Uses the supplied sid (`persist` is ignored) |
