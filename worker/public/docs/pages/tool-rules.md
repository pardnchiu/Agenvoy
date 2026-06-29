# Tool Design & Rules

## Tool design rules

The four mandatory rules for adding or editing tools (enforced by `/tool-reviewer`):

1. **Name is the only semantic carrier** — stub-tool first calls only see the name; description and params arrive on the second round
2. **Description serves parameter-call correctness only** — no usage manuals, trigger conditions, or comparisons with other tools
3. **English only** — Chinese only appears in user-facing handler return messages
4. **Optional fields must declare a `default`** — handlers still defend against nil/missing

Description length: a single verb-led sentence by default. Forbidden: trigger conditions ("Use when ..."), tool comparisons, downstream flow instructions, output schema details.

## Tool concurrency markers

Tools have two independent flags:

- `ReadOnly` — exempts from confirm gate when `agen cli` is in use
- `Concurrent` — opts into Pass 2 fan-out (parallel goroutine per call)

Adding `Concurrent: true` requires both "no side effects" and "upstream allows parallelism". The current concurrent set is documented in Core Concepts (three-pass tool concurrency).

## Tool timeout matrix

Each adapter has its own timeout, layered with the executor-side ceiling:

| Adapter | Default | Configurable | Where |
|---|---|---|---|
| Built-in (`toolRegister.Dispatch`) | 1 min | `Def.Timeout` per tool | tool registration |
| Script (`script_*`) | 5 min (300s) | `tool.json` `"timeout": <seconds>` | `extensions/scripts/<name>/tool.json` |
| API (`api_*`) | 60s | `doc.Endpoint.Timeout`; hard cap 300s | `extensions/apis/<name>.json` |
| MCP HTTP | 60s `http.Client.Timeout` + 1 min outer dispatch | n/a | MCP server config |
| MCP stdio | 1 min outer dispatch only | n/a | MCP server config |

Long-running tools (script + API) emit `running name=... elapsed=Ys/Zs` to the daemon log every 30s for visibility.

Subagent + external-agent tools have their own multi-minute caps (`invoke_subagent` = `MAX_SUBAGENT_TIMEOUT_MIN`, `invoke_external_agent` = 10 min, `cross_review_with_external_agents` = 15 min, `generate_plan` / `transcribe_media` = 5 min, `generate_image` = 15 min).

## Credential auto-heal

`store_secret` is `AlwaysLoad: true` so the agent sees it on the first round. When a downstream tool returns a missing-key or invalid-credential error (`401` / `403` / `invalid api key` / `expired token`), the system prompt's `§10 Credential auto-heal` SOP directs the agent to call `store_secret` (which captures the new value through masked input — the value never reaches the LLM) and retry the original tool. Capped at two `store_secret` rounds per failing tool per turn.
