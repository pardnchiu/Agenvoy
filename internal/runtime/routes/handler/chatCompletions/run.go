package chatCompletions

import (
	"cmp"
	"context"
	"os"
	"slices"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	execmodel "github.com/pardnchiu/agenvoy/internal/agents/exec/model"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

func run(ctx context.Context, req Request, userContent string, events chan<- agenttypes.Event) {
	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	trimContent := strings.TrimSpace(userContent)
	if trimContent != "" {
		events <- agenttypes.Event{Type: agenttypes.EventUserInput, Text: trimContent}
	}

	events <- agenttypes.Event{Type: agenttypes.EventAgentSelect}

	model := configbot.DefaultModel
	if trimModel := strings.TrimSpace(req.Model); trimModel != "" {
		model = trimModel
	}
	agent, fallbacks, reasoning, err := execmodel.Route(ctx, model, "", trimContent, "", "")
	if err != nil {
		events <- agenttypes.Event{Type: agenttypes.EventError, Err: err}
		return
	}
	events <- agenttypes.Event{Type: agenttypes.EventAgentResult, Text: strings.TrimSpace(agent.Name())}

	workDir := req.workDir
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}

	excludeTools := []string{"*"}
	if req.agentMode {
		excludeTools = tools.TUIOnlyTools
	}

	data := exec.ExecuteMeta{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		InputNoRecord:  trimContent,
		Reasoning:      cmp.Or(req.ReasoningEffort, reasoning, "medium"),
		ExcludeTools:   excludeTools,
		ExcludeSkills:  tools.TUIOnlySkills,
		ClientTools:    req.Tools,
		AllowAll:       true,
	}

	session := buildStatelessSession(req, trimContent, workDir, data.Agent.Name())

	if err := exec.Execute(ctx, data, session, events, true); err != nil {
		events <- agenttypes.Event{Type: agenttypes.EventError, Err: err}
	}
}

func buildStatelessSession(req Request, userInput, workDir string, model string) *agenttypes.Session {
	var systemPrompts []go_llm_router.Message
	if req.agentMode {
		systemPrompts = exec.BuildChatCompletionsSystemPrompts(workDir, model)
	}
	systemPrompts = append(systemPrompts, req.systemPrompts...)

	lastUserIdx := -1
	for i, one := range slices.Backward(req.Messages) {
		if one.Role == "user" {
			lastUserIdx = i
			break
		}
	}
	var oldHistories []go_llm_router.Message
	if lastUserIdx > 0 {
		oldHistories = append(oldHistories, req.Messages[:lastUserIdx]...)
	}

	return &agenttypes.Session{
		SystemPrompts:    systemPrompts,
		ContextHistories: oldHistories,
		ToolHistories:    []go_llm_router.Message{},
		ToolResults:      []go_llm_router.Message{},
		UserMessage:      go_llm_router.Message{Role: "user", Content: userInput},
		Stateless:        true,
	}
}
