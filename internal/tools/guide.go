package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var topicNames = []string{"tool_generate", "tool_error", "web_search", "market_analysis", "targeted_read", "ask_user", "subagent_dispatch", "write_todo", "html_render", "office", "figure"}

var topicGuides = map[string]string{
	"tool_generate":     configs.GuideToolGenerate,
	"tool_error":        configs.GuideToolError,
	"web_search":        configs.GuideWebSearch,
	"market_analysis":   configs.GuideMarketAnalysis,
	"targeted_read":     configs.GuideTargetedRead,
	"ask_user":          configs.GuideAskUser,
	"subagent_dispatch": configs.GuideSubagentDispatch,
	"write_todo":        configs.GuideWriteTodo,
	"html_render":       configs.GuideHtmlRender,
	"office":            configs.GuideOffice,
	"figure":            configs.GuideFigure,
}

func registReasoningGuide() {
	toolregister.Regist(toolregister.Def{
		Name:        "reasoning_guide",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default] Returns the full rules for the topics below; call before acting on any match, with every matching topic in one call:
- tool_error: a tool call just failed, before retrying it.
- tool_generate: live external data (weather, FX, stock, geocoding, translation...) no script_*/ext_* covers, or the one found fails — before public_api_list, http_request or edit_tool.
- web_search: non-smalltalk info query (people, orgs, facts, current events, prices).
- market_analysis: stock / ETF / market analysis or an entry decision.
- targeted_read: a file question needing only some symbols, sections or keywords.
- ask_user: missing target, vague scope or spec, ambiguous time, scheduling without content, more than one fitting tool.
- subagent_dispatch: one lookup across 3+ entities, one question needing 2+ independently searched source classes, a discovered set needing per-entity work, or a named session ("call X" / "呼叫 X") — before subagents(mode=invoke) or background_subagents.
- write_todo: analysis / research / evaluation, or work needing more than one round of tool calls — before the first tool call; an active Skill follows the Skill Execution Rules.
- html_render: an HTML deliverable, before writing any markup.
- office: creating or modifying .docx / .xlsx / .pptx, before writing the file.
- figure: an answer that carries a chart, flowchart, diagram or formula, before writing it.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topics": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string", "enum": topicNames},
					"minItems":    1,
					"description": "Every topic that matches this turn.",
				},
			},
			"required": []string{"topics"},
		},
		Handler: func(_ context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Topics []string `json:"topics"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			selection := ""
			guides := make([]string, 0, len(params.Topics))
			seen := make(map[string]bool, len(params.Topics))
			for _, one := range params.Topics {
				topic := strings.TrimSpace(one)
				if topic == "" || seen[topic] {
					continue
				}
				guide, ok := topicGuides[topic]
				if !ok {
					return "", utilsmsg.New(utilsmsg.Invalid, "topics", "unknown topic %q; available: %s", topic, strings.Join(topicNames, ", "))
				}
				if strings.Contains(guide, "{{.ModelSelection}}") {
					if selection == "" {
						selection = config.SelectionPrompt(&config.Config{}, true)
						if cfg, err := config.Load(); err == nil {
							selection = config.SelectionPrompt(cfg, true)
						}
					}
					guide = strings.ReplaceAll(guide, "{{.ModelSelection}}", selection)
				}
				guide = strings.NewReplacer(
					"{{.MaxLegs}}", strconv.Itoa(min(configs.MAX_CONCURRENT_TOOLS, configs.MAX_CONCURRENT_SUBAGENTS)),
					"{{.MaxBackground}}", strconv.Itoa(configs.MAX_BACKGROUND_TASKS),
					"{{.BackgroundMin}}", strconv.Itoa(configs.BACKGROUND_THRESHOLD_MIN),
				).Replace(guide)
				seen[topic] = true
				guides = append(guides, "## "+topic+"\n\n"+strings.TrimSpace(guide))
			}
			if len(guides) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "topics", "required; available: %s", strings.Join(topicNames, ", "))
			}
			return strings.Join(guides, "\n\n"), nil
		},
	})
}
