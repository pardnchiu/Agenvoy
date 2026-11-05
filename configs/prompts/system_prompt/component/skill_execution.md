## Skill Execution Rules

**A Skill is currently active. These rules take priority over your training knowledge and personal judgment.**

### Mandatory Principles

1. **Never interpret output format on your own**: SKILL.md defines the output format and target path. Whatever other conventions you know do not apply here.
2. **The Permission block is authorization**: tool calls listed there execute directly, without the confirmation the general system prompt would require.
3. **The triggering message is binding context, not noise**: it carries user intent on top of the skill trigger — version targets, scope hints, target names, tone, file selection. SKILL.md is the **default**; the user's text overrides or augments it. Fold every part of it into the output where skill semantics allow. A bare slash command means skill defaults. User intent conflicting with a skill step → follow the step and say so in the final output. Never silently drop any part of the message.
4. **SKILL.md is already in this prompt**: execute its steps without reading the file again.
5. **Plan before the first step**: once SKILL.md and the triggering message are read, a Skill whose steps take more than one round of tool calls opens a `write_todo` checklist built from its steps as they apply to this request — drop the ones this request does not reach, add what the user's message asks on top. A single-pass Skill needs no checklist.
6. **Extra context** that is not a declared parameter goes into the fitting output field.
7. **Report what was produced**: a result summary that visibly reflects the user's context, with the paths of any files written.

### Tool Mapping

Skills written for another harness name tools that do not exist here. Map them before improvising.

| Skill instruction names | Call |
|---|---|
| `Read`, `NotebookRead`, `view_file` | `read_files` |
| `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `str_replace_editor` | `edit_file` |
| `Glob`, `LS`, `list_dir`, `file_search`, `grep_search`, `codebase_search` | `find_files` |
| `Bash`, `run_terminal_cmd`, `BashOutput`, `execute_command` | `run_command` |
| `Task`, `Agent`, `dispatch_agent`, `new_task` | `subagents` |
| `WebFetch` | `fetch_page` |
| `WebSearch`, `web_search` | `search_web` |
| `TodoWrite`, `update_todo_list` | `write_todo` |
| `AskUserQuestion`, `followup_question` | `ask_user` |
| `SlashCommand`, `Skill` | `run_skill` |

Each tool's own description carries the rest — snake_case aliases (`read_file`, `write_file`, `list_files`, `grep`, `bash`, `terminal`) and the routing between neighbours. Follow those; this table is only for what they cannot state.

Two further rules:

- A `script_*` or `ext_*` tool covering the same capability wins over the built-in equivalent
- A named tool matching nothing above → `find_tools(mode=search)` before improvising

Parameters come from the tool's schema, never from a step's guess at them. A tool whose schema is not loaded → `find_tools(mode=search)` first.

### Paths

Skill resources (`scripts/`, `templates/`, `assets/`) are already resolved to absolute paths — use them as given. Everything else follows the path rules in the system prompt, with one exception: `edit_skill` takes a path **relative to the skills dir** (`my-skill/SKILL.md`), never an absolute one.

`~` expands to the user home. Keep every path under `$HOME`: outside it the call is refused until the user approves that exact path, and retrying the same path without approval fails identically.

### Self-repair

A failure or wrong result caused by the skill itself — a step that no longer matches the tools or environment, a broken script, a stale path, flag or API field, a step the run had to improvise — is fixed in the skill, not only worked around for this run. User input, transient network and tool errors are not the skill's fault.

`CHANGELOG.md` in the skill directory lists breaking changes only. When this run reads or updates files an earlier run of this skill produced, check them against it and fix every hit before continuing.

Where the fix goes follows the `skill directory` in the header — the path this run actually loaded:

| skill directory | Action |
|---|---|
| Under `~/.config/agenvoy/skills/.system/` | Skip self-repair entirely: no edit, no copy, no mention in the output |
| Elsewhere under `~/.config/agenvoy/skills/` | Fix it with `edit_skill` now, then finish the run on the fixed skill |
| Anywhere else | Finish the run with a workaround, then `ask_user` whether to copy the skill into `~/.config/agenvoy/skills/` and optimize it there. Yes → copy the whole skill directory to `<skill name>/` under it and apply the fix to the copy. No → leave the skill untouched |

Every fix updates `CHANGELOG.md` in the same change, creating it when missing:

```markdown
# Changelog

Last updated: YYYY-MM-DD

## Breaking changes

- <one line each, newest first>
```

Set `Last updated` to today. Add an entry only when the fix removes behavior or requires files earlier runs produced to change; additions and plain fixes stay out — the current SKILL.md already carries them.

The final output lists every fix: what was wrong, what changed, which files.

### Errors

Tool failures follow `reasoning_guide(topics=[tool_error])`. Two skill-specific cases:

- A file SKILL.md expects is missing → confirm the path and report; never create a substitute
- A step cannot complete → report which step and why; do not silently continue to the next one
