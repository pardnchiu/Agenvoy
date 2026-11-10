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
	registBackgroundSubagents()
}

func invokeProperties() map[string]any {
	return map[string]any{
		"task": map[string]any{
			"type":        "string",
			"description": "The subtask, written to stand on its own: the leg sees none of this conversation.",
		},
		"report_name": map[string]any{
			"type":        "string",
			"description": "Short kebab-case name naming the leg's job and entity, e.g. nvda-news, tsmc-earnings; the report is saved as temp-<report_name>.md, a taken name gets a random suffix, blank uses a random name.",
			"default":     "",
		},
		"self_id": map[string]any{
			"type":        "string",
			"description": "The self id of the existing non-temp session to run in, spelled exactly as subagents(mode=list) prints it; set it verbatim when the user delegates by name, otherwise leave EMPTY. Never invent a descriptive label: an unmatched value resolves to nothing and the run becomes a temp session anyway. Broad parallel fan-out stays anonymous.",
			"default":     "",
		},
		"model": map[string]any{
			"type":        "string",
			"description": "Required for invoke. Worker model for the leg: map the leg's one job to its work kind and take the first model reasoning_guide(topics=[subagent_dispatch]) lists for it; it overrides the model a `self_id` session is pinned to. `auto` defers to that session's pinned model, else the dispatcher picks from the task as for a normal request. An unregistered or `pass`-tier name is rejected.",
			"default":     "",
		},
		"reasoning": map[string]any{
			"type":        "string",
			"enum":        reasoningLevels,
			"default":     configs.REASONING_AUTO,
			"description": "Thinking depth; overrides a `self_id` session's own setting. Pass `low` for gathering, since depth multiplies across the fan-out; raise it only for a leg whose own written output must reason rather than gather. `auto` defers to that session's setting, else the dispatcher picks it at one extra dispatcher call per leg.",
		},
		"new": map[string]any{
			"type":        "boolean",
			"description": "Only matters when `self_id` resolves to a named session; a temp session always starts clean. true ignores that session's history, summary and past tool records for this run (nothing is deleted); false continues its earlier work. Omitted: false when `self_id` resolves, otherwise true.",
		},
		"system_prompt": map[string]any{
			"type":        "string",
			"description": "Extra role or constraints appended to the leg's system prompt, e.g. 'Cite a URL for every figure.'",
			"default":     "",
		},
		"exclude_tools": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Extra tool names to exclude on top of the fixed set, which cannot be lifted: subagents, background_subagents, edit_file, edit_tool, edit_skill, test_tool, write_result, html_template, generate*, open_file, schedules, store_secret, the TUI-only tools, and ask_user for background legs.",
			"default":     []string{},
		},
	}
}

func registSubagents() {
	properties := invokeProperties()
	properties["mode"] = map[string]any{
		"type":        "string",
		"enum":        []string{"invoke", "list"},
		"description": "invoke: run the task in a subagent session; every other parameter except self_id applies to invoke only. list: the sessions whose self id contains `self_id`, with their roles; run it when the user delegates by name, then invoke with the self id it prints. Omitted: task given → invoke, otherwise list.",
		"default":     "invoke",
	}
	properties["self_id"] = map[string]any{
		"type":        "string",
		"description": "mode=list: required, the delegated name to resolve (case-insensitive substring of the self id). mode=invoke: the self id of the existing non-temp session to run in, spelled exactly as mode=list prints it; set it verbatim when the user delegates by name, otherwise leave EMPTY. Never invent a descriptive label: an unmatched value resolves to nothing and the run becomes a temp session anyway. Broad parallel fan-out stays anonymous.",
		"default":     "",
	}

	toolregister.Regist(toolregister.Def{
		Name:        "subagents",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     time.Duration(configs.MAX_SUBAGENT_TIMEOUT_MIN) * time.Minute,
		Description: fmt.Sprintf(`Runs a subtask in its own session and waits for it (invoke), or lists named agents' self ids and roles (list); an invoke returns a usage header and the path of the saved report, which you read_files and relay, since "dispatched" is not an answer.
Use for "call X", "ask X to ...", "have X look into ..." (naming an agent is an order: dispatch to X, never answer it yourself), and for fan-out when one lookup repeats across 3+ entities or one question needs 2+ source classes searched independently ("research ...", "gather data on ...", "compile a report" across several targets or sources).
Legs very likely over %[2]d minutes, or "take your time", "let me know when it's done", "run it in the background" → background_subagents; protocol and model tiers → reasoning_guide(topics=[subagent_dispatch]). One job per leg, one call per leg, %[1]d at a time.`, min(configs.MAX_CONCURRENT_TOOLS, configs.MAX_CONCURRENT_SUBAGENTS), configs.BACKGROUND_THRESHOLD_MIN),
		Parameters: map[string]any{
			"type":       "object",
			"properties": properties,
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
				return invokeSubagent(ctx, e, params, false)
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
