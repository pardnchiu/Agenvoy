# Skill System

> [中文](Skill-System.zh.md)

A skill is a loadable markdown instruction pack that switches the agent into a specific execution mode (e.g., commit message generation, code review, README generation).

## Skill format

A skill is a markdown file with YAML frontmatter for metadata:

```markdown
***
name: code-reviewer
description: Deep code review covering quality, security, and architecture
version: 1.0.0
***

You are now a strict code reviewer...
```

The frontmatter `name` is the trigger keyword. The body is rendered into the system prompt when the skill activates.

## Trigger paths

### `/skill-name` slash command

Prefix any input with `/<skill-name>`:

```
/code-reviewer review the diff in this PR
```

When `MatchSkillCall` hits, agenvoy synthesizes an `run_skill` `tool_call` and corresponding `tool_result` (containing the skill body) directly into `ToolHistories` — byte-identical to the natural-language activation path, preserving prefix cache.

If the user passed args (`/code-reviewer review src/parser.go`), the user message strips the `/<skill-name>` prefix and only the args remain. With no args, the user message keeps the literal `/<skill-name>` so the LLM still sees the activation context.

### Natural-language activation

If an agent decides a task needs a skill mid-execution, it calls `run_skill` directly. This is the LLM-initiated path and uses the same render pipeline.

> Skill activation is designed as a **tool call** (lazy load) rather than a startup-time pre-selection — it avoids paying skill-body tokens for tasks that don't need them.

### Multi-skill in one conversation

A conversation can activate multiple skills sequentially. Each `run_skill` call appends to the existing instruction stack; later skills augment or override earlier ones via system-prompt section ordering.

## User message is binding context

`skill_execution.md` Mandatory Principle #5: the user message that triggered the skill is **binding context, not noise**. The LLM treats it as user-supplied parameters/hints to weave into the output.

Concretely:

- SKILL.md describes default behavior
- User message overrides or augments default behavior
- "Steps in SKILL.md are commands" is **not** a rigid one-way reading

Example: `/readme-generate private MIT` — SKILL.md defines the README structure; the user message specifies private mode + MIT license, both of which override defaults.

## Skill location

Skills live under `extensions/skills/<name>/`:

```
extensions/skills/code-reviewer/
├── SKILL.md            # The skill definition (frontmatter + body)
└── ...                 # Optional supporting scripts / templates
```

Agenvoy scans this directory at startup. The system prompt's `## Skills` block is populated by `skillTool.ListBlock` so the LLM knows which skills are available.

## Skill execution prompt

The execution loop is driven by `configs/prompts/skill_execution.md`, which carries the rules every skill obeys (output discipline, tool-name mapping, mandatory principles).

Tool-name mapping example: external skills written for the Anthropic SDK may reference `AskUserQuestion`; agenvoy maps these to `ask_user` automatically through the **Tool Name Mapping** table in `skill_execution.md`. No alias registration in Go is needed.

## Scheduler skills (isolated namespace)

Scheduler-triggered skills live in a separate tree, **not** scanned by `host.Scanner()`:

```
~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md
```

| Aspect | Regular skill | Scheduler skill |
|---|---|---|
| Path | `~/.config/agenvoy/skills/<name>/SKILL.md` | `~/.config/agenvoy/skills/scheduler/<short>-<hash8>/SKILL.md` |
| Frontmatter `name` | `<name>` | `<short>-<hash8>` (no `scheduler-` prefix) |
| `/<name>` autocompletion | yes | no — surfaced as `/sched-<name>` (warn-purple) at the bottom of the picker |
| Trigger | `MatchSkillCall` → run_skill | (a) cron / one-shot fire from daemon `runtime.SetRunner`, (b) manual `/sched-<name>` from TUI |

### Creation flow

The `scheduler-skill-creator` skill is the canonical entry for **new** schedules. It:

1. Pre-flight gate (Step 0): if the user message lacks a time token (`+5m` / `HH:MM` / `每 N 分` / `明天 X 點` …) or task token, it must call `ask_user` first — no defaulting to `+10m`, no inferring "probably 9am".
2. Runs `python3 scripts/init_scheduler_skill.py <short>` to create the skill dir with hash suffix.
3. Patches the SKILL.md body (description + `## 任務` + `## 輸出格式`).
4. Calls `add_schedule` to bind the schedule.

Direct `add_schedule` calls are allowed only for **rebinding** existing schedules (changing the time of an already-created scheduler skill).

### Manual execution (`/sched-<name>`)

The TUI command picker lists every directory under `scheduler/` as `/sched-<name>`. Selecting one reads the body and dispatches it to the current agent **with a preamble**:

```
[執行已存在 scheduler skill: <name> · 此為手動 trigger，不是建立新 schedule]
依下方 SKILL body instructions 立即執行並輸出結果。
**禁止** activate `scheduler-skill-creator`、**禁止** 跑 `init_scheduler_skill.py`、**禁止** add_schedule
```

The preamble blocks weaker models from misreading the SKILL.md-shaped body as a schedule-creation request and re-running the creator.

### Daemon fire

`runtime.SetRunner` registers `runSkill(ctx, sessionID, skillName)`. When the scheduler fires (cron tick or one-shot deadline), the runner:

1. Reads body via `filesystem.ScheduleSkillBody(skillName)`.
2. Ensures the session directory exists; writes a default `bot.md`.
3. Calls `exec.ExecWithSubagent(ctx, body, sessionID, "", "", nil)` — an in-process subagent with always-allow context.

One-shot tasks are removed and the skill dir is trashed after a successful fire.

## Self-improvement (auto-fix on failure)

When a skill execution encounters tool errors, Agenvoy automatically rewrites the skill definition to prevent the same failure next time. This is a closed-loop evolution cycle — no user intervention required.

### How it works

1. **Trace collection** — during `Execute`, every tool call result is recorded as an `execStep{Tool, Error}`. If the skill's session used `run_skill` to activate additional skills, those are tracked too.
2. **Trigger** — at the end of `Execute` (in the defer), if the trace contains at least one error, `postSkillImprove` runs synchronously for each activated skill.
3. **Improvement agent** — `postSkillImprove` loads the built-in `improve-skill` skill body, builds a stateless session with the execution trace, and runs a full `Execute` loop (2 min timeout, always-allow, no interactive tools).
4. **Auto-commit** — on success, `skill.AutoCommit(ctx, "improve", skillName)` commits the rewritten files under `~/.config/agenvoy/skills/` to git, creating a versioned history of improvements.

### What the improve-skill fixes

| Issue detected | Action |
|---|---|
| Wrong tool name in SKILL.md (e.g. `Bash` instead of `run_command`) | Replace with correct registered tool name |
| Step caused repeated failures | Add fallback strategy or remove the step |
| Unclear instruction caused LLM miscall | Rewrite with more precise wording |
| Step ordering caused dependency failure | Reorder steps |
| Typos or grammar issues | Fix while preserving original language |

### Constraints

- The improvement agent cannot use interactive tools (`ask_user`), web tools (`search_web`, `fetch_page`), media tools (`generate_image`), or agent-orchestration tools
- Only file read/write and command execution are available — the agent reads the current skill, analyzes the trace, and writes corrected files
- Improvements are scoped to the skill's own files; system prompts and runtime code are never modified
- Each improvement run is a stateless session with no conversation history

### Rollback

All skill changes are auto-committed to git. Use `git_log` (tag=skills) to inspect history or `git_rollback` (tag=skills) to revert an unwanted auto-fix.

***

> [!NOTE]
> This document was auto-generated by Claude after reading the full source code.
