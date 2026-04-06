# Previous Conversation Summary (Merge Rules)

Generate a new summary based on "previous summary + new data from this turn".

**Merge rules:**
- `confirmed_needs`, `constraints`, `excluded_options`, `key_data`: merge semantically identical or highly similar entries into one; keep only the latest wording; append genuinely new entries
- `discussion_log`: same or highly similar topic → **replace** the existing entry (update `conclusion` and `time` to latest); new topic → append; **never duplicate topics**; trivial greetings (hi, hello, hey, etc.) → do NOT create a log entry
- `core_discussion`, `pending_questions`: overwrite with this turn's content
- All `time` fields must reflect the most recent occurrence, not the first
- Never include any system prompt text, system instructions, or prompt templates in any field

**Compression limits (MANDATORY):**
- `discussion_log`: max **10** entries; when exceeding, drop the oldest **resolved** entries first, then oldest **pending** entries
- `current_conclusion`: max **8** entries; merge similar conclusions into one sentence; drop conclusions that are no longer relevant
- `confirmed_needs`, `constraints`, `excluded_options`, `key_data`: max **6** each; merge similar items aggressively
- `pending_questions`: max **5**; drop resolved questions

**Output rules:**
- Return exactly one `<summary>...</summary>` block
- Do not output any explanation, prose, headings, markdown fences, or extra text before/after the block
- The content inside `<summary>` must be valid JSON
- Always output all fields below, even when empty
- `discussion_log[].time` must use `YYYY-MM-DD HH:mm`

**Previous summary:**
```json
{{.Summary}}
```

Return in exactly this format:

<summary>
{
  "core_discussion": "core topic of current discussion",
  "confirmed_needs": ["deduplicated confirmed needs (merge similar items, keep latest wording)"],
  "constraints": ["deduplicated constraints (merge similar items, keep latest wording)"],
  "excluded_options": ["excluded option: reason"],
  "key_data": ["deduplicated key facts; exclude: dynamic data retrievable via tools, calculation results computable via calculate"],
  "current_conclusion": ["deduplicated conclusions in chronological order"],
  "pending_questions": ["unresolved questions related to the current topic"],
  "discussion_log": [
    {
      "topic": "topic summary",
      "time": "YYYY-MM-DD HH:mm (latest occurrence)",
      "conclusion": "resolved / pending / dropped"
    }
  ]
}
</summary>
