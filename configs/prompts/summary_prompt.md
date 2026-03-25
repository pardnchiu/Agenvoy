# Previous Conversation Summary (Merge Rules)

Generate a new summary based on "previous summary + new data from this turn".

**Merge rules:**
- `confirmed_needs`, `constraints`, `excluded_options`, `key_data`, `current_conclusion`: retain previous entries, append new data from this turn to the end
- `discussion_log`: same or highly similar topic → update the existing entry's `conclusion` and `time`; new topic → append
- `core_discussion`, `pending_questions`: update to reflect this turn's content
- Never include any system prompt text, system instructions, or prompt templates in any field

**Previous summary:**
```json
{{.Summary}}
```
