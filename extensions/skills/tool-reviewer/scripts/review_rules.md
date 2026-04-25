# Tool Design Review Rules

Mirrors the "Tool 設計檢查清單" in project `CLAUDE.md`. Use as the sole rubric — do not invent extra criteria.

---

## Rule 1 — Name is the only semantic carrier

**Why**: stub tool short-circuit hides description / parameters until the LLM's second turn. The first selection pass sees only the tool name.

**Pass**: name is unambiguous, distinguishable from siblings, conveys both *what* and *which* (e.g. which kind of search, which kind of edit), and uses the same verb / suffix shape as its same-domain siblings.

**Fail patterns** (deterministic checks marked with → rule code):
- Generic verbs (`process`, `handle`, `do`, `manage`, `execute`, `perform`, `dispatch`, `run`) → `R1_GENERIC_VERB`
- Mixed `_` / `-` separators in the same name (Agenvoy convention is snake_case) → `R1_MIXED_SEPARATOR`
- Dynamic Go identifier the parser can't resolve (use a literal or a same-file `const`) → `R1_DYNAMIC_NAME`
- Names that collide on prefix with sibling tools, forcing the LLM to read description to disambiguate (LLM judgment — use scanner's `name_clusters` as the comparison anchor)
- Names that bury the discriminator in the description (`verify` when `cross_review_with_external_agents` is meant)
- Verb redundancy where the second token is implied by the first (`patch_edit` — `patch` already means edit)
- Verb inconsistency within a sibling cluster (one tool uses `analyze_*` while every other cluster member uses `fetch_*`)
- Inconsistent suffix vocabulary across same-domain tools (`read_tool_error` vs `remember_error` vs `search_error_memory` — pick one shape)

**Examples**:
- ✅ `invoke_subagent` / ❌ `dispatch_internal`
- ✅ `cross_review_with_external_agents` / ❌ `verify`
- ✅ `search_conversation_history` / ❌ `search_history` (collides with git / shell history)
- ✅ `fetch_youtube_transcript` / ❌ `analyze_youtube` (verb mismatches sibling `fetch_*` cluster)
- ✅ `apply_patch` / ❌ `patch_edit` (verb redundancy)

When flagging, propose the better name. Cite the relevant sibling cluster from the scan's `name_clusters` output so the suggestion is anchored, not abstract.

---

## Rule 2 — Description only ensures correct invocation

**Why**: description is read on the second turn, after the tool has already been selected. Its job is to make the LLM fill parameters correctly — not to re-pitch the tool, not to gate selection (Rule 1's job), not to teach.

**Pass**: a single sentence (or two) describing what the tool does, written so the LLM knows what to put in each parameter slot.

**Fail patterns**:
- Numbered trigger conditions (`(1) ... (2) ... (3) ...`) — if invocation gating is needed, push it to the system prompt or rely on the name
- `**bold**` / markdown emphasis — adds tokens without changing meaning
- Multiple paragraphs (> 2)
- Output schema dumps (`Returns {"name", "path", ...}`)
- Tie-breakers vs other tools (`vs invoke_subagent: ...`, `prefer this over X`)
- Implementation details (`auto-skips .gitignore`, `uses readability under the hood`)
- Manual-style prose ("This tool will help you ...", "When you need to ...")

When flagging, propose the trimmed one-sentence version.

---

## Rule 3 — English only

**Why**: mixed-language descriptions create token noise and hurt smaller / multilingual provider models (Gemini, NVIDIA, etc.). Internal user-facing handler return strings may stay in their original language; the *tool definition* (description, parameter descriptions, enums) must be English.

**Pass**: all of `description`, `parameters[*].description`, `parameters[*].enum` text are ASCII / English.

**Fail patterns**:
- Any CJK / Hangul / Hiragana / Katakana codepoint in tool or parameter description
- Mixed bilingual descriptions (`Inspect a file 檢查檔案`)
- Full-width punctuation (`，` `。` `「」`) — even if surrounding text is English

When flagging, propose the English rewrite.

---

## Rule 4 — Optional fields require explicit `default`

**Why**: without `default`, the LLM has to guess what omission means. With `default`, the schema *itself* tells the model what happens when the field is dropped.

**Pass**:
- Every parameter NOT in `required[]` declares `"default": <value>`
- Every parameter IN `required[]` does NOT declare `default`

**Fail patterns**:
- Optional field with no `default`
- Required field with `default` (semantically contradictory — pick one)
- `default: null` used as a placeholder when a real default exists in the handler

Handler still owns nil / missing defense — never trust schema default to materialize at the call site.

---

## Severity

| Severity | Rules | Reason |
|---|---|---|
| **High** | R1 (name clarity), R2 (description scope) | Wrong tool gets selected or wrong parameters get filled |
| **Medium** | R3 (English), R4 (optional default) | Token waste / ambiguity but tool still callable |
| **Low** | Cosmetic — bold markdown alone, single numbered list | Stylistic; flag but don't escalate |

Promote Medium → High when multiple Medium violations stack on the same tool.
