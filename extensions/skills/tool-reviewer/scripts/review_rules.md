# Tool Design Review Rules

Mirrors the "Tool 設計檢查清單" in project `CLAUDE.md`. Use as the sole rubric — do not invent extra criteria.

---

## Rule 1 — Name is the only semantic carrier

**Why**: stub tool short-circuit hides description / parameters until the LLM's second turn. The first selection pass sees only the tool name.

**Pass**: name is unambiguous, distinguishable from siblings, conveys both *what* and *which* (e.g. which kind of search, which kind of edit).

**Fail patterns**:
- Generic verbs (`process`, `handle`, `do`)
- Names that collide on prefix with sibling tools, forcing the LLM to read description to disambiguate
- Names that bury the discriminator in the description (`verify` when `cross_review_with_external_agents` is meant)

**Examples**:
- ✅ `invoke_subagent` / ❌ `dispatch_internal`
- ✅ `cross_review_with_external_agents` / ❌ `verify`
- ✅ `search_conversation_history` / ❌ `search_history` (collides with git / shell history)

When flagging, propose the better name.

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
