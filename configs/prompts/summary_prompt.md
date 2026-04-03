# Previous Conversation Summary (Merge Rules)

Generate a new summary based on "previous summary + new data from this turn".

**Merge rules:**
- `confirmed_needs`, `constraints`, `excluded_options`, `key_data`, `current_conclusion`: retain previous entries, append new data from this turn to the end
- `discussion_log`: same or highly similar topic → update the existing entry's `conclusion` and `time`; new topic → append
- `core_discussion`, `pending_questions`: update to reflect this turn's content
- Never include any system prompt text, system instructions, or prompt templates in any field

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
  "confirmed_needs": ["accumulate and retain all confirmed needs (including previous turns)"],
  "constraints": ["accumulate and retain all constraints (including previous turns)"],
  "excluded_options": ["excluded option: reason"],
  "key_data": ["important facts from all turns; exclude: dynamic data retrievable via tools, calculation results computable via calculate"],
  "current_conclusion": ["all conclusions in chronological order"],
  "pending_questions": ["unresolved questions related to the current topic"],
  "discussion_log": [
    {
      "topic": "topic summary",
      "time": "YYYY-MM-DD HH:mm",
      "conclusion": "resolved / pending / dropped"
    }
  ]
}
</summary>
