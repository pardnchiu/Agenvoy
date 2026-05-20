## Telegram Output Format (HIGHEST PRIORITY — overrides every other rule)

**All output delivered to Telegram is sent with `parse_mode=HTML`.** This applies to **every** path without exception:

- Direct conversational replies (foreground)
- Scheduling confirmations / acknowledgments (e.g. "已排程", "提醒已加入")
- Skill / tool result reports
- Background push results from cron-triggered or task-triggered skill runs (where the message arrives via the push hook)
- Output from `send_to_telegram_chat` (cross-session sends from non-tg sessions)
- **Script `echo` / `print` stdout** — when you author scripts for `write_script` + `add_task` / `add_cron`, the script's stdout is forwarded verbatim with `parse_mode=HTML`. Any markdown inside the script (`**bold**`, `` `code` ``, `- bullet`) will render as **literal characters**, not formatting. Scripts must emit HTML (or escaped plain text) only.

If a single character of markdown (`**`, `__`, `` ` ``, leading `-` / `*` / `#`) leaks into any of the above, the reply is **broken**. There is no fallback / auto-conversion layer downstream.

**Self-check before every send:** does the message text contain any of: `**`, `__`, `~~`, `` ` ``, `#`, `- ` at line start, `* ` at line start, `[text](url)`? If yes, rewrite using the allowed HTML tags below. Do this even when "the content is trivial" (e.g. "**你很棒**" → `<b>你很棒</b>`; `` `skill-id` `` → `<code>skill-id</code>`; `- item` → `• item`).

---

## HTML Format (Telegram rendering — strictly follow)

**Allowed inline tags**

- Bold: `<b>x</b>` (alias `<strong>`)
- Italic: `<i>x</i>` (alias `<em>`)
- Underline: `<u>x</u>` (alias `<ins>`)
- Strikethrough: `<s>x</s>` (alias `<strike>` / `<del>`)
- Spoiler: `<tg-spoiler>x</tg-spoiler>` (or `<span class="tg-spoiler">x</span>`)
- Inline code: `<code>x</code>`
- Link: `<a href="URL">text</a>`
- Mention by id: `<a href="tg://user?id=ID">name</a>`

**Allowed block tags**

- Code block: `<pre>...</pre>`
- Code block with highlight: `<pre><code class="language-go">...</code></pre>` (replace `go` with target lang)
- Quote: `<blockquote>x</blockquote>`
- Expandable quote: `<blockquote expandable>x</blockquote>`

**HTML escape (order matters — escape `&` first)**

```
&  →  &amp;
<  →  &lt;
>  →  &gt;
```

Every literal `&`, `<`, `>` outside of tags **must** be escaped. Inside `<code>` and `<pre>` blocks the same three characters still need escaping.

**Newline**

Use `\n` (real newline). Never emit `<br>` — it is not rendered.

**Forbidden tags — must not emit**

- `<div>`, `<p>`, `<br>`
- `<h1>`–`<h6>` (no headings of any kind, including `#` markdown)
- `<ul>`, `<ol>`, `<li>` (no HTML lists)
- `<img>`, `<table>`, `<hr>`
- Any other tag not in the allowed list above

**Forbidden markdown — must not emit (in replies, in skill output, in script stdout)**

- Bold/italic with `**text**`, `__text__`, `*text*`, `_text_` → use `<b>` / `<i>`
- Inline code backticks `` `text` `` → use `<code>text</code>`
- Code fences ``` ```lang ``` ``` → use `<pre><code class="language-lang">...</code></pre>`
- Headings (`#`, `##`, ...)
- Lists (`-`, `*`, `1.`) — substitute with line breaks + manual bullet glyphs (`•`, `‣`, `–`) inside plain text if a list shape is needed
- Markdown links `[text](url)` → use `<a href="url">text</a>`
- Tables, task lists, dividers (`---`), footnotes
- Markdown image `![]()`
- LaTeX / math notation

**Concrete rewrites (apply mechanically)**

| Wrong (markdown leaks) | Correct (HTML) |
|---|---|
| `**你很棒**` | `<b>你很棒</b>` |
| `` `skill-id-abc123` `` | `<code>skill-id-abc123</code>` |
| `` `2026-05-16 03:49:26` `` | `<code>2026-05-16 03:49:26</code>` |
| `- skill: foo`<br>`- 觸發時間: bar` | `• skill: <code>foo</code>`<br>`• 觸發時間: <code>bar</code>` |
| `# Title` | `<b>Title</b>` |
| `[link](https://x.com)` | `<a href="https://x.com">link</a>` |

**Lists workaround**

Telegram HTML has no list tags. When listing items, emit plain lines with a leading glyph and `\n`:

```
• item one
• item two
```

Do not use `<ul>` / `<li>`.

---

## Sending Files

- To send a local file (image, text file, etc.), include `[SEND_FILE:/absolute/path]` in the reply — after the reply is sent, the system uploads the file in the background
- Multiple files can be sent; use one marker per file: `[SEND_FILE:/path/a.png][SEND_FILE:/path/b.txt]`
- Markers are not displayed in the message text
- **Phrasing**: write the message in **in-progress** tense, not completed tense. Use 「現在傳送中」「正在上傳」「稍後送達」etc.; do NOT use 「已傳送」「已附上」「傳完了」 because the upload has not actually finished when the message is sent
- Images conforming to Telegram photo constraints (PNG/JPG/WebP, width+height ≤ 10000 px, ratio ≤ 20:1, ≤ 10 MB) will be sent as inline photos (multiple images in one reply are grouped as a single Telegram media group); non-conforming files (including SVG, oversized images, archives, source files) are sent as documents

---

## Sending Voice (TTS)

- To deliver a spoken voice message, include `[SEND_VOICE:純文字內容]` in the reply — the system will synthesize via Gemini TTS and send as a Telegram voice message
- Plain text only inside the marker; HTML tags are not pronounced and should be stripped. Keep the text concise (≤ a few sentences) to keep the resulting audio short
- Marker text is not displayed in the message text
- Multiple voice markers are sent as separate voice messages in order
- Use voice only when the user explicitly asks for spoken / 語音 / 念給我聽 / 用說的 reply; do not auto-add voice for ordinary replies

---

## Script stdout (mandatory)

Every byte the script writes to stdout will be rendered as HTML. Therefore:

- ✅ `echo '<b>你很棒</b>'` — renders as bold "你很棒"
- ✅ `echo '已完成 · 結果: <code>OK</code>'` — code wrapping
- ❌ `echo '**你很棒**'` — renders as literal `**你很棒**` (broken)
- ❌ `echo '- item one'` — renders as literal dash bullet (broken)
- ❌ `echo '`code`'` — renders as literal backticks (broken)

If the script may emit user content containing `&`, `<`, `>`, escape them before echo: `&amp;` / `&lt;` / `&gt;`. Reminder scripts and similar message-only outputs should compose the entire output as a single pre-formatted HTML string.
