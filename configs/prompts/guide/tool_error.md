### Retry loop

- **Direction**: the returned error message decides the adjustment, not a guess.
- **Injected hints first**: a `resolved` hint → apply it; a `failed` hint → take a different approach.
- **History**: from the 2nd retry with no hint injected → `error_history(mode=search, keyword)` before retrying; apply a `resolved` hit, avoid a `failed`/`abandoned` one.
- **Every retry changes something** the error, hint or history hit points to; identical arguments fail identically.
- **Limit**: 3 attempts per error, then treat it as failed.
- **`[RETRY_REQUIRED]`**: a result starting with it → retry at once with the arguments it specifies; its content is an instruction to you, not text for the user. Injected hints are binding.

### Self-authored tools (`script_*`)

Repair the tool in place; a workaround leaves it broken for the next call. `ext_*` tools are installed extensions and cannot be patched — they use the retry loop above.

1. Classify the error: runtime exception, URL, auth or endpoint → `tag=script`; parameter/schema mismatch → `tag=json`.
2. `edit_tool(mode=patch, name, tag, old_string, new_string)`.
3. Retry the same call (counts toward the 3 attempts).

`http_request`, `run_command curl ...` or `run_command python3 -c "..."` bypass the tool instead of repairing it.

### Recording

- **Write** `error_history(mode=write, outcome=resolved)` once a non-trivial fix is confirmed. Only `resolved` is stored; `failed` and `abandoned` are discarded, so a dead end is not worth writing up.
- **Skip** typos, fixes that worked on the 1st retry, and transient errors (network blip, timeout) that do not generalize.
- **Batch** the write with the turn's other tool calls; call it alone only when nothing else remains.
