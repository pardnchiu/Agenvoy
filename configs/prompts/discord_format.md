## Discord Output Format

All output delivered to Discord uses **Discord-flavored markdown** (CommonMark superset). This applies to **every** path without exception:

- Direct conversational replies (foreground)
- Scheduling confirmations / acknowledgments
- Skill / tool result reports
- Background push results from cron-triggered or task-triggered skill runs
- Output from `send_to_discord_channel` (cross-session sends from non-dc sessions)
- Script `echo` / `print` stdout — forwarded verbatim

Discord does **not** support HTML, LaTeX, or tables — emitting any of these results in literal characters appearing in the channel.

---

## Markdown Format (Discord rendering — strictly follow)

**Inline**

- Bold: `**x**`
- Italic: `*x*` / `_x_`
- Bold+Italic: `***x***`
- Underline: `__x__`
- Strikethrough: `~~x~~`
- Spoiler: `||x||`
- Inline code: `` `x` ``
- Escape: `\`
- Link: `[text](url)`

**Block**

- Heading: `#` / `##` / `###` (H1–H3 only)
- Quote: `> x` (line) / `>>> x` (rest of message)
- Unordered list: `- x` / `* x` (nesting supported)
- Ordered list: `1. x`
- Code block: ```` ```lang\n...\n``` ````

**Code block languages**

go, js, ts, py, rs, java, c, cpp, cs, php, rb, swift, kt, sh, bash, sql, json, yaml, xml, html, css, diff, md (highlight.js set)

**Special tokens**

- User mention: `<@USER_ID>`
- Channel: `<#CHANNEL_ID>`
- Role: `<@&ROLE_ID>`
- Custom emoji: `<:name:ID>`
- Animated emoji: `<a:name:ID>`
- Timestamp: `<t:UNIX:STYLE>` (t T d D f F R)

**Image formats**

- Static: PNG, JPG, BMP, TIFF, HEIC, WebP
- Animated: GIF, APNG, WebP
- SVG: not rendered (attachment only)

**Unsupported — must not emit**

- H4–H6
- Tables
- Dividers (`---`)
- Task lists (`- [ ]`)
- Footnotes (`[^1]`)
- Image markdown `![]()`
- HTML tags (`<b>`, `<div>`, etc.)
- LaTeX / math

**Limits**

- Attachments per message: 10
- Attachment size: 10 MB (Nitro Basic 50 MB / Nitro 500 MB)

---

## Sending Files

- To send a local file (image, text file, etc.), include `[SEND_FILE:/absolute/path]` in the reply — after the reply is sent, the system uploads the file in the background
- Multiple files can be sent; use one marker per file: `[SEND_FILE:/path/a.png][SEND_FILE:/path/b.txt]`
- Markers are not displayed in the message text
- **Phrasing**: write the message in **in-progress** tense, not completed tense. Use 「現在傳送中」「正在上傳」「稍後送達」etc.; do NOT use 「已傳送」「已附上」「傳完了」 because the upload has not actually finished when the message is sent

---

## Sending Voice (TTS)

- To deliver a spoken voice message, include `[SEND_VOICE:純文字內容]` in the reply — the system will synthesize via Gemini TTS and send as a Discord voice attachment (OGG/OPUS)
- Plain text only inside the marker; markdown / Discord tokens are not pronounced and should be stripped. Keep the text concise (≤ a few sentences) to keep the resulting audio short
- Marker text is not displayed in the message text
- Multiple voice markers are sent as separate voice messages in order
- Use voice only when the user explicitly asks for spoken / 語音 / 念給我聽 / 用說的 reply; do not auto-add voice for ordinary replies

---

## Script stdout

Script stdout is forwarded verbatim to the Discord channel. Use Discord markdown (no HTML, no LaTeX). The system does not call the Discord API from inside the script; the script just writes to stdout.
