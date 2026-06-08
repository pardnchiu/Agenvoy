package exec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

type execStep struct {
	Tool  string
	Error string
}

func postSkillImprove(s *skill.Skill, trace []execStep) {
	if s == nil || s.Name == "" {
		return
	}

	scanner := agents.Scanner()
	if scanner == nil || scanner.Skills == nil {
		return
	}
	improveSkill, ok := scanner.Skills.ByName["improve-skill"]
	if !ok || improveSkill == nil || improveSkill.Content == "" {
		slog.Debug("postSkillImprove: improve-skill not found in scanner")
		return
	}

	registry := agents.Registry()
	if len(registry.Entries) == 0 {
		return
	}
	agent := registry.Registry[registry.Entries[0].Name]
	if agent == nil {
		return
	}

	task := buildImproveTask(s, trace)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx = context.WithValue(ctx, allowAllCtxKey{}, true)

	workDir, _ := os.UserHomeDir()
	if workDir == "" {
		workDir = "/"
	}

	excludedTools := []string{
		"invoke_subagent", "invoke_external_agent",
		"cross_review_with_external_agents", "review_result",
		"ask_user", "generate_image", "search_web", "fetch_page",
	}
	excludedTools = append(excludedTools, tools.TUIOnlyTools...)

	execData := ExecData{
		Agent:         agent,
		Skill:         improveSkill,
		WorkDir:       workDir,
		Content:       task,
		ExcludeTools:  excludedTools,
		ExcludeSkills: tools.TUIOnlySkills,
		AllowAll:      true,
	}

	userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), task)

	session := &agentTypes.AgentSession{
		Stateless:     true,
		SystemPrompts: BuildSystemPrompts(workDir, "", scanner, "", true, execData.ExcludeSkills),
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		Histories:     []agentTypes.Message{},
		UserInput:     agentTypes.Message{Role: "user", Content: userText},
	}

	events := make(chan agentTypes.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(ctx, execData, session, events, true)
		close(events)
	}()

	for range events {
	}

	if err := <-errCh; err != nil {
		slog.Warn("postSkillImprove execute",
			slog.String("skill", s.Name),
			slog.String("error", err.Error()))
		return
	}

	slog.Info("postSkillImprove completed",
		slog.String("skill", s.Name))
}

func buildImproveTask(s *skill.Skill, trace []execStep) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Skill: %s\nSource: %s\n\n", s.Name, s.Path)

	sb.WriteString("## Execution Trace\n")
	for i, step := range trace {
		fmt.Fprintf(&sb, "%d. `%s`", i+1, step.Tool)
		if step.Error != "" {
			fmt.Fprintf(&sb, " → error: %s", step.Error)
		} else {
			sb.WriteString(" → ok")
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
