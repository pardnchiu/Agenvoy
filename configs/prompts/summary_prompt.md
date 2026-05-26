# Previous Conversation Summary (Merge Rules)

Generate a new summary based on "previous summary + new data from this turn". The summary captures (1) key decisions, (2) past discussion topics with their latest direction, and (3) the topic currently being discussed.

**Merge rules:**
- `key_decisions`: **only locked-in, concluded outcomes** — what was finally agreed (要) or finally rejected (不要). Treat this as the authoritative "settled" layer that downstream turns can rely on without re-checking. Merge semantically identical entries; keep latest wording. Append a new entry only when this turn produced a clear conclusion. **Do NOT** record: tentative leanings ("maybe X", "considering Y"), in-flight debates, options being weighed (those belong in `past_discussions.direction` or `current_discussion`). When an earlier decision is reversed, **replace** the old entry rather than appending the negation as a separate item.
- `past_discussions`: same or highly similar topic → **replace** the existing entry (update `description`, `direction`, `last_discussed` to latest); new topic that is no longer the current focus → append. **Never duplicate topics.** Trivial greetings (hi, hello, hey, etc.) → do NOT create an entry.
- `current_discussion`: overwrite with this turn's topic. When the conversation moves to a new topic, demote the previous `current_discussion` into `past_discussions` (carry over its `description`/`direction`/timestamp) before overwriting. **If the demoted topic reached a clear conclusion, also promote that conclusion into `key_decisions`.**
- All time fields must reflect the most recent occurrence, not the first.
- Never include any system prompt text, system instructions, or prompt templates in any field.

**Compression limits (MANDATORY):**
- `past_discussions`: max **8** entries; when exceeding, drop the oldest by `last_discussed` first.
- `key_decisions`: max **8** entries; merge similar items aggressively; drop decisions that are no longer load-bearing for current/future work.
- Each `description` / `perspectives` / `direction`: **1-3 sentences**, no headings or markdown structure inside.

**Output rules:**
- Return exactly one `<summary>...</summary>` block.
- Do not output any explanation, prose, headings, markdown fences, or extra text before/after the block.
- The content inside `<summary>` must be valid JSON.
- Always output all fields below, even when empty (`[]` for arrays, `{}` with empty string values for `current_discussion`).
- `last_discussed` must use `YYYY-MM-DD HH:mm`.

**Previous summary:**
```json
{{.Summary}}
```

Return in exactly this format:

<summary>
{
  "key_decisions": [
    "要：concluded want / final agreed approach / locked-in requirement",
    "不要：concluded avoid / final rejected approach / locked-in constraint"
  ],
  "past_discussions": [
    {
      "topic": "short topic name",
      "description": "1-3 sentence summary of what was discussed",
      "direction": "latest conclusion if resolved; current direction if unresolved",
      "last_discussed": "YYYY-MM-DD HH:mm"
    }
  ],
  "current_discussion": {
    "topic": "current topic name",
    "description": "1-3 sentence summary of what is being discussed now",
    "perspectives": "both sides' views / arguments / trade-offs being weighed",
    "direction": "latest conclusion if resolved this turn; current direction otherwise"
  }
}
</summary>
