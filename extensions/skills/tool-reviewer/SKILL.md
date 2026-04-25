---
name: tool-reviewer
description: Audit Agenvoy tool definitions (built-in Go tools, extensions/apis/*.json, extensions/scripts/*/tool.json) against the project's tool design rules — name clarity, description scope, English-only text, and explicit defaults on optional fields. Use when the user wants to review tool quality, check tool design compliance, or audit api_/script_ extensions.
---

# Tool Reviewer

Audits all Agenvoy tool definitions against the four design rules in project `CLAUDE.md` and emits a violation report.

## Sources Audited

| Source | Path | Tool name prefix |
|---|---|---|
| Built-in | `internal/tools/**/*.go` (any `toolRegister.Regist` block) | (varies) |
| API extensions | `extensions/apis/*.json` | `api_*` |
| Script extensions | `extensions/scripts/*/tool.json` | `script_*` |

## Rules (mirrors `CLAUDE.md` "Tool 設計檢查清單")

1. **Name is the only semantic carrier** — LLM filters tools by name first (stub short-circuit hides description / parameters until the second turn). Name must be self-explanatory and unambiguous against sibling tools (e.g. `search_conversation_history` ≻ `search_history`).
2. **Description only ensures correct invocation** — not a manual, not a trigger checklist, not a tool-vs-tool comparison. Strip:
   - Numbered trigger conditions (`(1) ... (2) ...`)
   - `**bold**` emphasis / markdown decoration
   - Output schema dumps (`{"name", "path", ...}`)
   - Tie-breakers vs other tools (`vs invoke_subagent: ...`)
   - Implementation details (`auto-skip .gitignore`, `uses readability`)
3. **English only** — both tool description and parameter descriptions must be English. CJK / mixed-language is a violation. (User-facing handler return messages may stay in Chinese.)
4. **Optional fields require explicit `default`** — every parameter not in `required[]` must declare `"default": <value>` so the LLM knows the omission semantics. Required fields must NOT carry `default`.

## Command Syntax

```
/tool-reviewer [PROJECT_PATH] [OUTPUT_FILE]
```

| Parameter | Default | Description |
|---|---|---|
| `PROJECT_PATH` | Current directory | Agenvoy repo root |
| `OUTPUT_FILE` | `.doc/tool-reviewer/{yyyy-MM-dd_HH-mm}.md` | Report output (relative to `PROJECT_PATH`) |

### Examples

```bash
/tool-reviewer                       # → .doc/tool-reviewer/2026-04-25_14-30.md
/tool-reviewer .                     # same
/tool-reviewer . custom.md           # explicit override
```

## Workflow

```
1. Scan       →  python3 scripts/scan_tools.py {PROJECT_PATH}
                 outputs JSON:
                   { tools: [{source, name, description, parameters, required, file, line}, ...],
                     deterministic_violations: [{tool, rule, detail}, ...],
                     name_clusters: { <first_token>: [<tool>, ...] }   ← anchor for R1 sibling review
                   }

2.A R1 sweep  →  Walk EVERY tool returned by the scan and write a one-line R1 verdict
                 (`pass` or `fail` + suggested rename). This step is mandatory — there is
                 no "skip if name looks fine" branch. Use `name_clusters` to compare
                 each tool against its siblings (same first token). Failing patterns:
                   • Generic verb (`process`, `handle`, `dispatch`, `verify`, ...)
                   • Verb inconsistency with siblings in the same cluster
                     (e.g. `analyze_X` when every other cluster member is `fetch_*`)
                   • Verb redundancy (`patch_edit` — `patch` already implies edit)
                   • Description carries semantic load that should be in the name
                     (e.g. `verify` whose description says "cross-review with external agents"
                     → rename to `cross_review_with_external_agents`)
                   • Inconsistent suffix vocabulary across a cluster
                     (e.g. `read_tool_error` / `remember_error` / `search_error_memory`
                     — same domain, three different shapes)
                 Verdicts are emitted in the report's `## Name Audit` section (see output_format.md).

2.B R2 sweep  →  Same enumeration discipline for description scope. Re-read each description
                 and ask "is this content needed *to fill the parameters correctly*?" If
                 the answer is "no, it's there to *decide whether to call the tool*", flag
                 it as a Rule 2 violation and suggest the trimmed version.

3. Gate       →  if zero deterministic + zero heuristic violations across all tools, skip Save
                 and print a one-line "no issues" message. Honor explicit OUTPUT_FILE override.
                 The Name Audit section must still be produced inside the report when one is
                 written, even if all verdicts are `pass` — coverage > brevity.

4. Save       →  mkdir -p {PROJECT_PATH}/.doc/tool-reviewer/ then write the report.
```

## Deterministic Checks (handled by `scripts/scan_tools.py`)

The script flags these without LLM judgment — the LLM only needs to confirm and add context:

| Check | Trigger |
|---|---|
| `R1_DYNAMIC_NAME` | `Name:` field is a Go identifier the parser could not resolve to a same-file `const` literal |
| `R1_MIXED_SEPARATOR` | Tool name contains both `_` and `-` (Agenvoy convention is snake_case) |
| `R1_GENERIC_VERB` | Name's first token is a generic verb (`do`, `process`, `handle`, `manage`, `execute`, `perform`, `dispatch`, `run`); see `GENERIC_VERB_WHITELIST` for justified exceptions |
| `R3_NON_ENGLISH_DESCRIPTION` | Tool description contains any CJK / Hangul / Hiragana / Katakana codepoint |
| `R3_NON_ENGLISH_PARAM` | Parameter description contains any CJK codepoint |
| `R4_OPTIONAL_NO_DEFAULT` | Parameter is not in `required[]` AND has no `default` key |
| `R4_REQUIRED_HAS_DEFAULT` | Parameter is in `required[]` AND has a `default` key (semantically wrong) |
| `R2_BOLD_MARKDOWN` | Description contains `**...**` or `__...__` |
| `R2_NUMBERED_TRIGGER` | Description contains `(1)`, `1.`, `2.` style enumerations |
| `R2_MULTI_PARAGRAPH` | Description has > 2 blank-line-separated paragraphs |
| `R2_TOOL_COMPARISON` | Description contains `vs `, ` vs.`, `prefer over `, `instead of <other_tool>` |

The scanner also emits `name_clusters` (tools grouped by first token after stripping `api_` / `script_` prefix) so the LLM-side R1 sweep has a concrete sibling list per cluster.

## Heuristic Checks (LLM judgment)

For **every** tool the script returns — no skipping — apply these checks. Coverage is enforced by the Validation Checklist below: every tool must appear with a verdict in the report's `## Name Audit` section.

- **Name quality (R1)**: would another LLM, seeing only the name, correctly choose this tool over its siblings? Use `name_clusters` from the scan output as the comparison anchor — same first token = sibling group. Suggest a better name on fail. Failure modes:
  - Generic verb the deterministic checker missed (e.g. `verify`, `query`, `inspect` when not specific enough)
  - Verb inconsistency within a cluster (one tool uses `analyze_*` while every sibling uses `fetch_*`)
  - Verb redundancy (`patch_edit`, `delete_remove_*`)
  - Name buries the discriminator in description (`verify` whose description reveals it actually means `cross_review_with_external_agents`)
  - Inconsistent suffix vocabulary across same-domain tools (`read_tool_error` / `remember_error` / `search_error_memory` — pick one shape)
- **Description scope drift (R2)**: re-read each description and ask "is this content needed *to fill the parameters correctly*?" If the answer is "no, it's there to *decide whether to call the tool*", flag it as a Rule 2 violation and suggest the trimmed version.
- **Parameter description bloat**: parameter descriptions repeating the tool description, explaining philosophy (e.g. path resolution rules), or listing edge cases — flag as Rule 2 violation.

## Reference Files

| Step | File | Purpose |
|---|---|---|
| Evaluate | [`scripts/review_rules.md`](scripts/review_rules.md) | Full rule definitions with positive / negative examples |
| Save | [`scripts/output_format.md`](scripts/output_format.md) | Report structure template |

## Validation Checklist

- [ ] All three sources scanned (built-in / api_ / script_)
- [ ] Every deterministic violation in the JSON appears in the report
- [ ] **Name Audit section present and lists EVERY tool** (count must equal `summary.tool_count`); each row has an explicit `pass` / `fail+suggestion` verdict — no tool may be silently omitted
- [ ] Every tool also received a description scope review (R2) — failures land in the per-source detail sections, passes are implied by absence
- [ ] `name_clusters` from the scan output were consulted (cite at least one cluster comparison in any R1 fail entry)
- [ ] Suggestions are concrete (proposed new name, proposed trimmed description), not abstract advice
- [ ] No-Op gate respected — if zero violations and no explicit `OUTPUT_FILE`, skip the file (the gate covers detail sections; if a report IS written, the Name Audit section is mandatory)
- [ ] Report grouped by source → severity → tool, not flat
