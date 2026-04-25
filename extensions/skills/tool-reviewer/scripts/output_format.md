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

## Built-in Tools

### `<tool_name>` · `<file>:<line>`

- **Severity**: High / Medium / Low
- **Rules failed**: R1, R2, R4
- **Findings**:
  - R1 — Name clarity: <description of issue>. **Suggested name**: `<new_name>`.
  - R2 — Description scope: <which content should be removed>. **Suggested description**:
    ```
    <one-line trimmed version>
    ```
  - R4 — Optional `<param>` has no default. **Suggested**: add `"default": <value>`.

(repeat per tool with violations; omit clean tools)

## API Extensions (`api_*`)

(same structure)

## Script Extensions (`script_*`)

(same structure)

## Clean Tools

One-line list of tools that passed all four rules:

- `tool_a` · `path/to/file:line`
- `tool_b` · `path/to/file:line`
```

## Writing Rules

1. **One section per source** (Built-in / API / Script). Within each, sort tools by severity descending then by name.
2. **Skip clean tools** in detailed sections. Only list them in the final "Clean Tools" roll-up.
3. **Every finding gets a concrete suggestion** — proposed name, proposed trimmed description, exact `default` value to add. No abstract advice ("consider clarifying").
4. **Quote source where useful** — for description scope violations, paste the original (truncated to 200 chars) and the proposed rewrite side-by-side so the reader can diff at a glance.
5. **Group multiple findings on the same tool** — one tool block lists all its rule failures together, not one block per rule.
6. **Severity rules** (from `review_rules.md`):
   - High: R1 fail, R2 fail, or 2+ Medium stacked on one tool
   - Medium: R3 fail, R4 fail (single)
   - Low: cosmetic only (e.g. lone `**bold**` with otherwise clean description)
7. **No-Op message** (when total violations = 0 and `OUTPUT_FILE` not explicitly given):
   ```
   Tool review clean: {N} tools across builtin/api/script meet all four design rules.
   ```

## What NOT to write

- Do not invent rules outside the four in `review_rules.md`.
- Do not lecture about general API design — only check against the rules.
- Do not propose breaking changes (renames) without flagging them as breaking.
- Do not include the full deterministic-violation JSON in the report — summarize.
