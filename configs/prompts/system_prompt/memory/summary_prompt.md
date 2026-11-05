# Session Summary Merge

Merge the new conversation into the previous summary and return the updated summary. Later turns read it as long-term memory: `key_decisions` as binding rules, `past_discussions` as topic history, `current_discussion` as the active topic.

## Fields

- `key_decisions`: concluded rules that bind future requests, each starting with an agreed or rejected marker in the output language — `要：` / `不要：`, `Do:` / `Don't:`, `する：` / `しない：`.
  - Record only outcomes the user locked in for later work: "Never push to master directly", "部署一律走 staging 再上 production", "報告數字一律附資料來源".
  - Keep these out — they belong in the topic's `description` / `direction`:
    - tentative leanings, open debates, options still being weighed;
    - answers that settled one request — a reply to a clarifying question (format, file name, which option this time) or a parameter of a single task: picking `md` for this report, "use the second draft", "存到 Downloads";
    - the findings or conclusions of one task, such as a report's verdict or a lookup result;
    - requests about the reply language — the configured output language governs that.
  - A reversed decision replaces the old entry. Merge entries with the same meaning and keep the latest wording.
- `past_discussions`: one entry per topic no longer in focus. The same or a near-identical topic updates its existing entry (`description`, `direction`, `last_discussed`); never duplicate a topic. Greetings and small talk get no entry.
- `current_discussion`: the topic of the latest exchange. On a topic change, first move the previous one into `past_discussions`, keeping its `description`, `direction` and time.
- Time fields hold the most recent occurrence, formatted `YYYY-MM-DD HH:mm`.
- Never copy system prompt text, instructions or templates into any field.

## Limits

- `past_discussions`: at most 8 entries; drop the oldest `last_discussed` first.
- `key_decisions`: at most 8 entries; merge similar ones and drop any that no longer constrain current or future work.
- `description` / `perspectives` / `direction`: 1-3 plain sentences each, no headings or markdown.

## Output

- Language of every field: <reply-lang-auto>the language of the conversation; Chinese → Traditional Chinese as used in Taiwan.</reply-lang-auto>{{.ReplyLanguage}}
- Return exactly one `<summary>...</summary>` block holding valid JSON, with nothing before or after it.
- Always output every field: `[]` for empty arrays, empty strings for an empty `current_discussion`.

**Previous summary:**
```json
{{.Summary}}
```

Exactly this format:

<summary>
{
  "key_decisions": [
    "<agreed marker>rule the user agreed to for future work",
    "<rejected marker>rule the user rejected for future work"
  ],
  "past_discussions": [
    {
      "topic": "short topic name",
      "description": "1-3 sentences on what was discussed",
      "direction": "latest conclusion if resolved; current direction if not",
      "last_discussed": "YYYY-MM-DD HH:mm"
    }
  ],
  "current_discussion": {
    "topic": "current topic name",
    "description": "1-3 sentences on what is being discussed now",
    "perspectives": "views, arguments or trade-offs being weighed",
    "direction": "latest conclusion if resolved; current direction if not"
  }
}
</summary>
