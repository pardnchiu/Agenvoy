A hook is a Python script Agenvoy runs at a fixed point of every task. It receives that point's data as JSON on stdin and answers whether the task may continue.

### Pick the state

The state name is also the file name; read_hook and write_hook only accept these names.

- Block a phrase, cap a token budget → `before_user_send` / `before_tool_call`.
- Log or alert on tool results, failures or answers → `after_tool_call` / `after_tool_error` / `after_ask_user`.
- Notify or record when a task ends → `after_task_done` / `after_task_cancel`.

### Write it

1. Call `read_hook(name=<state>)` first. The demo's `main(data)` shows every field that state receives; use only those fields.
2. When `current` shows an installed hook, each state has one file, so add the new rule with `write_hook(mode=patch, name=<state>)`, copying `old_string` from that script, instead of replacing what it already does.
3. When `current` is `(none)` there is nothing to patch; create it with `write_hook(mode=write, name=<state>)`, keeping the demo's stdin parsing (including empty stdin) and its final print. `write_hook` writes without asking; do not use write_file for hooks.
4. Run `test_hook(name=<state>, input=...)` with input that exercises the new rule, not run_command. Loop until it is right: an error or an unexpected `success` → fix it with `write_hook(mode=patch)` → `test_hook` again. Stop once it returns the expected `success` and message.

### Output

The last stdout line must be `{"success": 1|0, "debug": "...", "error": "..."}` with all three fields:

- Pass: `success: 1`; a non-empty `debug` is shown to the user as the hook's result.
- Block: `success: 0` with the reason in `error`. `before_tool_call` skips that call and returns the tool name and reason to the agent as its result; `after_tool_call` keeps the result and sends the agent a steer message with the tool name and reason; in both the task carries on. `before_user_send`, `after_tool_error` and `after_ask_user` stop the task and show `Cancel: <State>: <error>`. `after_task_done` / `after_task_cancel` run after the task has ended, so blocking there has no effect.
- Earlier stdout lines and stderr go to the debug log only; use them for tracing.
- A crash, a non-JSON last line, or running past 30 seconds counts as a block.

The script runs sandboxed in the task's work directory and may only write under the home directory, never inside ~/.config/agenvoy (read-only); keep it to standard-library Python.
