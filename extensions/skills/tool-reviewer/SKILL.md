---
name: tool-reviewer
description: Audit Agenvoy tool definitions (built-in Go tools, extensions/apis/*.json, extensions/scripts/*/tool.json) against the project's tool design rules under the lazy-schema model — name clarity, description trigger coverage, schema field completeness, English-only text, and explicit defaults on optional fields. Use when the user wants to review tool quality, check tool design compliance, or audit api_/script_ extensions.
---

# Tool Reviewer

Audits all Agenvoy tool definitions against the four design rules in project `CLAUDE.md` and emits a violation report.

## Sources Audited

| Source | Path | Tool name prefix |
|---|---|---|
| Built-in | `internal/tools/**/*.go` (any `toolRegister.Regist` block) | (varies) |
| API extensions | `extensions/apis/*.json` | `api_*` |
| Script extensions | `extensions/scripts/*/tool.json` | `script_*` |

## Rules (mirrors `CLAUDE.md` "Tool 註冊" contract — lazy-schema model)

**Lazy-schema context**: every tool's `name` + `description` is always in LLM context; the `parameters` JSON schema is replaced with a stub `{"type":"object","properties":{}}` unless `AlwaysLoad=true`. The LLM decides whether to invoke a tool (or call `search_tools` to load its schema) using `name` + `description` alone. Schema is the call contract, loaded on demand.

1. **Name is self-explanatory** — verb + noun, direct, distinguishable from siblings (e.g. `search_chat_history` ≻ `search_history`). Name is the anchor; description elaborates the trigger.
2. **Description describes WHEN to invoke — maximally concise and precise** — one or two sentences covering trigger signals + trade-offs against similar tools, then stop. This is the LLM's only signal before the schema loads, but bloat dilutes the trigger. Required content (kept tight):
   - Trigger conditions (when the tool applies; what user intents map to it)
   - Contrast with similar tools when one exists (`prefer over X when Y`)
   What to avoid (filler dilutes selection signal):
   - Padding phrases ("This tool allows you to...", "Use this when needed...")
   - `**bold**` / markdown emphasis (token waste; structure ≠ semantics)
   - Output schema dumps (belongs in `parameters` / response examples, not selection text)
   - Implementation gossip (`uses readability under the hood`) unrelated to selection
   - Call-contract details (type/unit/enum/default) — those belong in `parameters[*].description`
   - Per-use-case bullet lists when a single sentence covers them
   Target length 60–200 chars. Going past three sentences usually means schema content leaked in.
3. **Schema fields are complete call contracts** — every entry in `parameters.properties` must carry a `description` covering: type, unit, accepted values (enum/regex), edge cases, interaction with other params, and at least one concrete example when the type is non-trivial (e.g. cron expression, file path with placeholders, JSON body shape).
4. **English only** — `description`, `parameters[*].description`, `enum` text must be English. CJK / mixed-language is a violation. (User-facing handler return strings may stay in Chinese.)
5. **Optional fields require explicit `default`** — every parameter not in `required[]` must declare `"default": <value>` so the LLM knows the omission semantics. Required fields must NOT carry `default`.

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
                     (e.g. `read_tool_error` / `remember_error` / `search_error_history`
                     — same domain, three different shapes)
                 Verdicts are emitted in the report's `## Name Audit` section (see output_format.md).

2.B R2 sweep  →  Same enumeration discipline for description trigger coverage. Re-read each
                 description and ask "could another LLM, seeing only this description, know
                 WHEN to call this tool?" If the description only states what the tool
                 executes ("Fetches RSS feed") without trigger signals or use cases, flag
                 as R2 violation and suggest the expanded version with trigger context.

2.C R3 sweep  →  For each tool, walk every `parameters.properties` entry. Missing
                 `description`, single-word descriptions, or non-trivial types (cron
                 expression / file path with placeholders / JSON body shape) without a
                 concrete example → flag as R3 violation.

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
| `R2_SHORT_DESC` | Description shorter than 60 characters — too thin to convey trigger signals under the lazy-schema model |
| `R2_BOLD_MARKDOWN` | Description contains `**...**` or `__...__` (token waste; structure ≠ semantics) |
| `R3_PARAM_NO_DESC` | A `properties` entry has no `description` field (or empty/whitespace-only) |
| `R3_PARAM_SHORT_DESC` | Parameter description shorter than 20 characters AND type is non-trivial (`object`, `array`, or has `enum`) — likely missing examples/constraints |
| `R4_NON_ENGLISH_DESCRIPTION` | Tool description contains any CJK / Hangul / Hiragana / Katakana codepoint |
| `R4_NON_ENGLISH_PARAM` | Parameter description contains any CJK codepoint |
| `R5_OPTIONAL_NO_DEFAULT` | Parameter is not in `required[]` AND has no `default` key |
| `R5_REQUIRED_HAS_DEFAULT` | Parameter is in `required[]` AND has a `default` key (semantically wrong) |

Note: rules `R2_NUMBERED_TRIGGER`, `R2_MULTI_PARAGRAPH`, `R2_TOOL_COMPARISON` from earlier versions have been removed — trigger enumeration, multi-paragraph trigger context, and tool comparisons are now expected description content, not violations.

The scanner also emits `name_clusters` (tools grouped by first token after stripping `api_` / `script_` prefix) so the LLM-side R1 sweep has a concrete sibling list per cluster.

## Heuristic Checks (LLM judgment)

For **every** tool the script returns — no skipping — apply these checks. Coverage is enforced by the Validation Checklist below: every tool must appear with a verdict in the report's `## Name Audit` section.

- **Name quality (R1)**: would another LLM, seeing only the name, correctly choose this tool over its siblings? Use `name_clusters` from the scan output as the comparison anchor — same first token = sibling group. Suggest a better name on fail. Failure modes:
  - Generic verb the deterministic checker missed (e.g. `verify`, `query`, `inspect` when not specific enough)
  - Verb inconsistency within a cluster (one tool uses `analyze_*` while every sibling uses `fetch_*`)
  - Verb redundancy (`patch_edit`, `delete_remove_*`)
  - Name buries the discriminator in description (`verify` whose description reveals it actually means `cross_review_with_external_agents`)
  - Inconsistent suffix vocabulary across same-domain tools (`read_tool_error` / `remember_error` / `search_error_history` — pick one shape)
- **Description trigger coverage AND concision (R2)**: re-read each description and ask two questions: "could another LLM, seeing only this, know WHEN to call this tool?" AND "is anything here filler that doesn't aid selection?" Flag as R2 violation when EITHER fails. Look for:
  - Action-only descriptions ("Lists files", "Sends HTTP request") with no trigger context — under-spec
  - Missing contrast when sibling tools exist (`search_tools` vs `list_tools` — when to use which) — under-spec
  - Missing scenario hints for tools whose name alone doesn't carry the use case — under-spec
  - Over three sentences, paragraph-form prose, bullet enumerations — over-spec (bloat dilutes trigger; usually call-contract content leaked in from `parameters`)
  - Filler ("This tool allows you to...", "Useful for various tasks") that adds no selection signal — over-spec
  - Output format dumps, response shape descriptions — belongs in `parameters` / response example, not selection text
  Propose a tightened rewrite (60-200 chars, one or two sentences) for every fail.
- **Parameter schema completeness (R3)**: walk every `properties` entry. Flag as R3 violation when:
  - `description` missing or single-word
  - Non-trivial type (`object`, `array`, cron expression, file path with placeholders, JSON body shape) without a concrete example
  - `enum` listed without explaining what each value means
  - Interaction with other parameters not documented (e.g. "required when `mode=advanced`")
  - Unit not specified for numeric values (seconds vs milliseconds, bytes vs MiB)

## Reference Files

| Step | File | Purpose |
|---|---|---|
| Evaluate | [`scripts/review_rules.md`](scripts/review_rules.md) | Full rule definitions with positive / negative examples |
| Save | [`scripts/output_format.md`](scripts/output_format.md) | Report structure template |

## Validation Checklist

- [ ] All three sources scanned (built-in / api_ / script_)
- [ ] Every deterministic violation in the JSON appears in the report
- [ ] **Name Audit section present and lists EVERY tool** (count must equal `summary.tool_count`); each row has an explicit `pass` / `fail+suggestion` verdict — no tool may be silently omitted
- [ ] Every tool also received a description trigger-coverage review (R2) and a parameter schema completeness review (R3) — failures land in the per-source detail sections, passes are implied by absence
- [ ] `name_clusters` from the scan output were consulted (cite at least one cluster comparison in any R1 fail entry)
- [ ] Suggestions are concrete (proposed new name, proposed trimmed description), not abstract advice
- [ ] No-Op gate respected — if zero violations and no explicit `OUTPUT_FILE`, skip the file (the gate covers detail sections; if a report IS written, the Name Audit section is mandatory)
- [ ] Report grouped by source → severity → tool, not flat
