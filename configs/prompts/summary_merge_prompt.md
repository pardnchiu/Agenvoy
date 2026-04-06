You have two conversation summaries. Merge them into one deduplicated summary.

Rules:
- Semantically identical or highly similar entries: keep only the latest wording
- discussion_log: same topic → keep the entry with the latest time; new topic → keep both; max **10** entries, drop oldest resolved first
- current_conclusion: max **8** entries, merge similar conclusions
- confirmed_needs, constraints, excluded_options, key_data: max **6** each, merge similar items
- pending_questions: max **5**, drop resolved questions
- All time fields must use the most recent date
- Remove any redundant or duplicate entries
- Output exactly one `<summary>...</summary>` block with valid JSON, no extra text

**Old summary:**
```json
{{.OldSummary}}
```

**New summary:**
```json
{{.NewSummary}}
```
