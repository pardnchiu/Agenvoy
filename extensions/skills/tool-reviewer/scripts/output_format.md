# Output Format

```markdown
# Tool Review — {YYYY-MM-DD HH:mm}

> Repo: `{PROJECT_PATH}` · Tools scanned: {N} (builtin: {a} / api: {b} / script: {c})
> Violations: {total} (high: {h} / medium: {m} / low: {l})

## Summary

| Source | Tools | Clean | Violations |
|---|---:|---:|---:|
| Built-in | … | … | … |
| API extensions | … | … | … |
| Script extensions | … | … | … |

## Name Audit

> Mandatory: row count must equal `summary.tool_count` from the scanner. No tool may be omitted.
> One row per tool, grouped by source then by sibling cluster (use `name_clusters` from the scan output).
> Verdict is `✓` (pass) or `✗` (fail) — fail rows include the suggested rename and the reason.

### Built-in (cluster: `fetch_*`)

| Tool | Verdict | Suggested rename | Reason |
|---|:---:|---|---|
| `fetch_page` | ✓ | — | — |
| `fetch_google_rss` | ✓ | — | — |
| `analyze_youtube` | ✗ | `fetch_youtube_transcript` | Verb breaks `fetch_*` cluster shape; tool actually fetches transcript |

### Built-in (cluster: `read_*`)

| Tool | Verdict | Suggested rename | Reason |
|---|:---:|---|---|
| `read_file` | ✓ | — | — |
| … | … | … | … |

(repeat for every cluster; singletons go in a final `(singletons)` table)

## Built-in Tools

### `<tool_name>` · `<file>:<line>`

- **Severity**: High / Medium / Low
- **Rules failed**: R1, R2, R4
- **Findings**:
  - R1 — Name clarity: <description of issue, citing sibling cluster if relevant>. **Suggested name**: `<new_name>`.
  - R2 — Description scope: <which content should be removed>. **Suggested description**:
    ```
    <one-line trimmed version>
    ```
  - R4 — Optional `<param>` has no default. **Suggested**: add `"default": <value>`.

(repeat per tool with violations; omit clean tools — they are already covered by Name Audit)

## API Extensions (`api_*`)

(same structure: Name Audit cluster tables → per-tool detail blocks)

## Script Extensions (`script_*`)

(same structure)

## Clean Tools

One-line list of tools that passed all four rules:

- `tool_a` · `path/to/file:line`
- `tool_b` · `path/to/file:line`
```

## Writing Rules

1. **One section per source** (Built-in / API / Script). Within each, sort tools by severity descending then by name.
2. **Name Audit comes first** — before any per-source detail block. Total row count must equal `summary.tool_count`. Verdicts cite sibling clusters from `name_clusters` so reviewers can reproduce the call.
3. **Skip clean tools** in detailed sections. Only list them in the final "Clean Tools" roll-up. Clean tools are still enumerated in Name Audit.
4. **Every finding gets a concrete suggestion** — proposed name, proposed trimmed description, exact `default` value to add. No abstract advice ("consider clarifying").
5. **Quote source where useful** — for description scope violations, paste the original (truncated to 200 chars) and the proposed rewrite side-by-side so the reader can diff at a glance.
6. **Group multiple findings on the same tool** — one tool block lists all its rule failures together, not one block per rule.
7. **Severity rules** (from `review_rules.md`):
   - High: R1 fail, R2 fail, or 2+ Medium stacked on one tool
   - Medium: R3 fail, R4 fail (single)
   - Low: cosmetic only (e.g. lone `**bold**` with otherwise clean description)
8. **No-Op message** (when total violations = 0 and `OUTPUT_FILE` not explicitly given):
   ```
   Tool review clean: {N} tools across builtin/api/script meet all four design rules.
   ```
   When this branch fires, no Name Audit table is written either — the no-op message replaces the entire report.

## What NOT to write

- Do not invent rules outside the four in `review_rules.md`.
- Do not lecture about general API design — only check against the rules.
- Do not propose breaking changes (renames) without flagging them as breaking.
- Do not include the full deterministic-violation JSON in the report — summarize.
