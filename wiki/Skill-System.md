# Skill System

> [中文](https://github.com/agenvoy/Agenvoy/wiki/Skill-系統)

A skill is a loadable markdown instruction pack that switches the agent into a specific execution mode (e.g., commit message generation, code review, README generation).

## Skill format

A skill is a markdown file with YAML frontmatter for metadata:

```markdown
---
name: code-reviewer
description: Deep code review covering quality, security, and architecture
version: 1.0.0
---

You are now a strict code reviewer...
```

The frontmatter `name` is the trigger keyword. The body is rendered into the system prompt when the skill activates.

## Trigger paths

### `/skill-name` slash command

Prefix any input with `/<skill-name>`:

```
/code-reviewer review the diff in this PR
```

When `MatchSkillCall` hits, agenvoy synthesizes an `activate_skill` `tool_call` and corresponding `tool_result` (containing the skill body) directly into `ToolHistories` — byte-identical to the natural-language activation path, preserving prefix cache.

If the user passed args (`/code-reviewer review src/parser.go`), the user message strips the `/<skill-name>` prefix and only the args remain. With no args, the user message keeps the literal `/<skill-name>` so the LLM still sees the activation context.

### Natural-language activation

If an agent decides a task needs a skill mid-execution, it calls `activate_skill` directly. This is the LLM-initiated path and uses the same render pipeline.

> Skill activation is designed as a **tool call** (lazy load) rather than a startup-time pre-selection — it avoids paying skill-body tokens for tasks that don't need them.

### Multi-skill in one conversation

A conversation can activate multiple skills sequentially. Each `activate_skill` call appends to the existing instruction stack; later skills augment or override earlier ones via system-prompt section ordering.

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
