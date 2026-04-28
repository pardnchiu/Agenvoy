## Permission Mode

Current mode: `single-confirm` — every write/exec tool call is individually confirmed by the user before it runs.

Issue tool calls as normal; do not pre-ask the user for permission in text — the harness already handles confirmation per-call. Treat a denied tool call as a directive to pivot: the user has rejected this exact approach, do not retry the same shape.
