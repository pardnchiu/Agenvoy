package chatCompletions

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

func run(ctx context.Context, req Request, userContent string, events chan<- agentTypes.Event) {
	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	trimContent := strings.TrimSpace(userContent)
	if trimContent != "" {
		events <- agentTypes.Event{Type: agentTypes.EventUserInput, Text: trimContent}
	}

	events <- agentTypes.Event{Type: agentTypes.EventAgentSelect}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	registry := agents.Registry()
	switch {
	case req.Model == "" || req.Model == "auto":
		primary, rest, err := exec.ResolveAgent(ctx, agents.DispatcherBot(), registry, trimContent, false, "")
		if err != nil {
			events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
			return
		}
		agent = primary
		fallbacks = rest

	default:
		a, ok := registry.Registry[req.Model]
		if !ok {
			events <- agentTypes.Event{Type: agentTypes.EventError, Err: fmt.Errorf("model %q not found", req.Model)}
			return
		}
		agent = a
	}
	events <- agentTypes.Event{Type: agentTypes.EventAgentResult, Text: strings.TrimSpace(agent.Name())}

	workDir := req.workDir
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}
	data := exec.ExecData{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Content:        trimContent,
		ExcludeTools:   tools.TUIOnlyTools,
		ExcludeSkills:  tools.TUIOnlySkills,
		AllowAll:       true,
	}

	session := buildStatelessSession(req, trimContent, workDir, scanner, data.ExcludeSkills)

	if err := exec.Execute(ctx, data, session, events, true); err != nil {
		events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
	}
}

func buildStatelessSession(req Request, userInput, workDir string, scanner *runtime.SkillScanner, excludeSkills []string) *agentTypes.AgentSession {
	systemPrompts := exec.BuildChatCompletionsSystemPrompts(workDir, scanner, excludeSkills)
	systemPrompts = append(systemPrompts, req.systemPrompts...)

	lastUserIdx := -1
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	var oldHistories []agentTypes.Message
	if lastUserIdx > 0 {
		oldHistories = append(oldHistories, req.Messages[:lastUserIdx]...)
	}

	wrappedUser := fmt.Sprintf("---\n當前時間: %s\n工作目錄: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), workDir, userInput)

	return &agentTypes.AgentSession{
		SystemPrompts: systemPrompts,
		OldHistories:  oldHistories,
		Histories:     append([]agentTypes.Message{}, oldHistories...),
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		UserInput:     agentTypes.Message{Role: "user", Content: wrappedUser},
		Stateless:     true,
	}
}
