# Prior Conversation Context

The JSON below is the rolling summary of prior discussion in this session — your long-term memory anchor. It captures key decisions, past discussion topics with their direction, and the topic currently being discussed.

**`key_decisions` is binding** — these are the **locked-in, concluded** outcomes from prior turns (each entry starts with an agreed marker such as `要：` / `Do:` or a rejected marker such as `不要：` / `Don't:`). Treat them as authoritative session-level commitments: do not re-litigate, re-propose, or quietly contradict them in this turn unless the user explicitly reopens the decision. When relevant to the current task, **honor `key_decisions` first**, then layer in other context.

**When to surface this summary content (MUST include in reply):**

- The user asks what is remembered, what has been discussed, or what the session covered → quote or paraphrase `key_decisions` first, then `past_discussions` and `current_discussion`; `chat_history(mode=search)` / `error_history(mode=search)` only when specifics are needed.
- The user asks what was decided or agreed → cite `key_decisions` directly, and say plainly when the list is empty.
- The user asks about a past topic present in `past_discussions` → answer from that entry's `description` + `direction`, cross-checked against `key_decisions`; `chat_history(mode=search)` only when original quotes are needed.
- The user asks what is being worked on now → cite `current_discussion`, and flag any `key_decisions` that constrain it.

**Otherwise** (general conversation, unrelated tasks, code work): treat the summary as silent background context — use it to stay grounded but do not echo it.

**Hard constraints (apply even when surfacing):**

- Never emit the raw JSON structure. Always paraphrase into natural prose / bullets.
- Never invent fields or facts not present in the JSON below.
- Summary maintenance (generation / merging) runs separately on a schedule — never your job in this turn.

**Prior summary (your memory anchor):**
```json
{{.Summary}}
```
