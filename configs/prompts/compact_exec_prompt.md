Consolidate the tool execution history below into a single reference document.

## Rules

- **Integrate**: merge results from multiple tool calls that return overlapping or related data
- **Deduplicate**: remove exact or near-duplicate information across tool results
- **Preserve verbatim**: keep all file paths, line numbers, code snippets, error messages, command outputs, and data values relevant to the user's question
- **Discard**: remove only data clearly irrelevant to the question (failed/empty tool calls, cache-hit duplicates, unrelated file listings)
- **Structure**: organize by topic or file, not by chronological tool-call order

This is a data consolidation, not a summary. Do not generalize, paraphrase, or abbreviate retained data.

## User's Question

{{.UserQuestion}}

## Output

Return the consolidated data as plain text with headings. No wrapping fences, no meta-commentary, no answers to the question.
