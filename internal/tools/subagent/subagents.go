package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Register() {
	registSubagents()
}

func registSubagents() {
	toolregister.Regist(toolregister.Def{
		Name:        "subagents",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     time.Duration(configs.MAX_SUBAGENT_TIMEOUT_MIN) * time.Minute,
		Description: fmt.Sprintf(`Runs a subtask in its own session and returns the path of its saved report (invoke), or lists named agents' self ids (list).
Naming an agent is an order: 呼叫 X / 請 X / 找 X / call X / ask X → dispatch to X, never answer it yourself.
Also fan out when one lookup repeats across 3+ entities, or one question needs 2+ source classes searched independently (查資料 / 研究 / 整理報告 across several targets or sources).
The report lands at ~/.config/agenvoy/download/temp-<report_name>.md; read_files it and relay it, since "已呼叫" is not an answer.
One job per leg, one call per leg, %d at a time (background: at most %d per session). Protocol and model tiers → reasoning_guide(topic=subagent_dispatch).`, min(configs.MAX_CONCURRENT_TOOLS, configs.MAX_CONCURRENT_SUBAGENTS), configs.MAX_BACKGROUND_TASKS),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"invoke", "list"},
					"description": "invoke: run the task in a subagent session. list: the sessions whose self id contains `self_id`, with their roles — run it when the user delegates by name, then invoke with the self id it prints. Omitted: task → invoke, otherwise list.",
					"default":     "invoke",
				},
				"task": map[string]any{
					"type":        "string",
					"description": "mode=invoke: the subtask, written to stand on its own — the leg sees none of this conversation. Its result comes back as a header [subagent · <model> · session=<id> · usage: ...] plus the path of the report file, and that usage line is the leg's whole token cost, to be tallied across every fan-out call when reporting this turn's cost.",
				},
				"report_name": map[string]any{
					"type":        "string",
					"description": "mode=invoke: short kebab-case name for the report file, naming the leg's job and entity (e.g. nvda-news, tsmc-earnings); saved as temp-<report_name>.md. Taken names get a random suffix; blank uses a random name.",
					"default":     "",
				},
				"self_id": map[string]any{
					"type":        "string",
					"description": "mode=list: required, the delegated name to resolve; only sessions whose self id contains it come back. mode=invoke: the self id of the existing non-temp session to run in, spelled exactly as mode=list prints it — set it verbatim when the user delegates by name, otherwise leave EMPTY. Never invent a descriptive label: an unmatched value resolves to nothing and the run becomes a temp session anyway. Broad parallel fan-out stays anonymous.",
					"default":     "",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "mode=invoke: required. Worker model for the leg: map the leg's one job to its work kind and take the first model the tier list in reasoning_guide(topic=subagent_dispatch) gives for it; it overrides the model a `self_id` session is pinned to. `auto` defers to that session's pinned model, else the dispatcher picks from the task as for a normal request. Any other unregistered or `pass`-tier name is rejected.",
					"default":     "",
				},
				"reasoning": map[string]any{
					"type":        "string",
					"enum":        reasoningLevels,
					"default":     configs.REASONING_AUTO,
					"description": "mode=invoke: thinking depth; overrides a `self_id` session's own setting. Pass `low` for gathering — depth multiplies across the fan-out; raise it only for a leg whose own written output must reason rather than gather. `auto` defers to that session's setting, else the dispatcher picks it, costing one extra dispatcher call per leg.",
				},
				"new": map[string]any{
					"type":        "boolean",
					"description": "mode=invoke: start the leg with a clean slate — prior conversation history, summary and past tool records of whatever session it lands in are ignored for this run (nothing is deleted). Omitted: true when no session is named, false when `self_id` resolves to one. Pass false to continue a session's earlier work — including a temp session whose previous leg failed, so the new leg sees what was already gathered.",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "mode=invoke: extra role or constraints appended to the subagent's system prompt.",
					"default":     "",
				},
				"background": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": fmt.Sprintf("mode=invoke. true when every leg is very likely to take over %d minutes end to end (multi-source research, broad data gathering), or the user says 慢慢查 / 跑完告訴我 / 背景跑. Use it only for the gathering that opens a request, dispatching every background leg in one go: the turn continues at once, and each report later resumes this task with only the objective and that report, none of this turn's tool results, so anything done before a late dispatch is lost to the resume. false: a leg of a few calls, or one whose result decides your next tool call; waiting for every report before a final synthesis does not count. Same value on every leg of one fan-out; stateless requests reject true.", configs.BACKGROUND_THRESHOLD_MIN),
				},
				"timeout_min": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("mode=invoke with background: true only. Minutes before the leg is stopped and reported as failed, 1-%d. Set it near the expected research time so a stuck leg fails early instead of holding the task for the full window; a foreground leg is capped at %d minutes regardless.", configs.MAX_BACKGROUND_TIMEOUT_MIN, configs.MAX_SUBAGENT_TIMEOUT_MIN),
					"minimum":     1,
					"maximum":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
					"default":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
				},
				"exclude_tools": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "mode=invoke: extra tool names to exclude on top of the always-excluded set (subagents, edit_file, every generate_* tool, and the TUI-only tools). The default set cannot be overridden.",
					"default":     []string{},
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params invokeParams
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			mode := strings.TrimSpace(params.Mode)
			if mode == "" {
				mode = "list"
				if strings.TrimSpace(params.Task) != "" {
					mode = "invoke"
				}
			}

			switch mode {
			case "invoke":
				return invokeSubagent(ctx, e, params)
			case "list":
				if strings.TrimSpace(params.SelfID) == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "self_id", "required when mode=list")
				}
				return listSessions(params.SelfID), nil
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown mode %q; available: invoke, list", mode)
		},
	})
}
