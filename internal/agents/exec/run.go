package exec

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	agentsexecmodel "github.com/pardnchiu/agenvoy/internal/agents/exec/model"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	sessionconfigbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Prepare(data ExecuteMeta) ExecuteMeta {
	if !data.TUI {
		data.ExcludeTools = append(append([]string{}, tools.TUIOnlyTools...), data.ExcludeTools...)
		data.ExcludeSkills = append(append([]string{}, tools.TUIOnlySkills...), data.ExcludeSkills...)
	}

	scanner := data.SkillScanner
	if scanner == nil {
		scanner = agents.Scanner()
	}
	if scanner != nil {
		scanner.Scan()
		if data.Skill == nil && data.SkillName == "" {
			if matched, effective := runtime.MatchSkill(scanner, data.Task, data.ExcludeSkills...); matched != nil {
				data.Skill = matched
				data.Task = strings.TrimSpace(effective)
				data.Input = data.Task
			}
		}
	}
	return data
}

func Run(ctx context.Context, data ExecuteMeta, events chan<- agentstypes.Event, isSub bool) error {
	sessionID := strings.TrimSpace(data.SessionID)
	if sessionID == "" {
		return utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}

	windowHash := agentstypes.WindowHash(ctx)
	if data.PendingTask != "" && !interactive.HasBackground(sessionID, data.PendingTask) && sessionlog.LastTaskHash(sessionID) != data.PendingTask {
		renamed := go_pkg_utils.UUID()
		if err := interactive.RenamePending(sessionID, data.PendingTask, renamed); err != nil {
			return utilsmsg.Trace("interactive.RenamePending", data.PendingTask, err)
		}
		if err := historystore.RenameFileTask(ctx, sessionID, data.PendingTask, renamed); err != nil {
			slog.Warn("historystore.RenameFileTask",
				slog.String("session", sessionID),
				slog.String("task", data.PendingTask),
				slog.String("error", err.Error()))
		}
		data.PendingTask = renamed
	}
	taskHash := data.PendingTask
	if taskHash == "" {
		if data.TaskHash == "" {
			data.TaskHash = go_pkg_utils.UUID()
		}
		taskHash = data.TaskHash
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	registerCancel(taskHash, cancel)
	defer unregisterCancel(taskHash)

	if name := data.SkillName; name != "" && data.Skill == nil {
		if slices.Contains(data.ExcludeSkills, name) {
			return utilsmsg.New(utilsmsg.Denied, "skill", "%q not available in this session", name)
		}
		scanner := data.SkillScanner
		if scanner == nil {
			scanner = agents.Scanner()
		}
		if scanner != nil {
			data.Skill = scanner.Lookup(name)
		}
		if data.Skill == nil {
			return utilsmsg.New(utilsmsg.NotFound, "skill", "%q", name)
		}
	}

	if data.Skill != nil {
		skillResult := agentstypes.Event{Type: agentstypes.EventSkillResult, Text: strings.TrimSpace(data.Skill.Name), TaskHash: taskHash, WindowHash: windowHash}
		events <- skillResult
		sessionlog.Record(sessionID, skillResult)
	}

	if input := strings.TrimSpace(data.Input); input != "" {
		if IsSchedule(ctx) {
			sessionlog.Append(sessionID, windowHash, "[Scheduled Task: "+DCPushPrefix(ctx)+"]")
		} else {
			events <- agentstypes.Event{Type: agentstypes.EventUserInput, Text: input, TaskHash: taskHash, WindowHash: windowHash}
			sessionlog.Append(sessionID, windowHash, input)
		}
	}

	var (
		agent     agentstypes.Agent
		fallbacks []agentstypes.Agent
	)
	if isSub {
		if data.Model == "" {
			data.Model = sessionconfigbot.DefaultModel
		}
		if !strings.HasPrefix(sessionID, "temp-") {
			sessModel, sessReasoning := sessionconfigbot.GetModel(sessionID)
			if data.Model == sessionconfigbot.DefaultModel {
				data.Model = sessModel
			}
			if data.Reasoning == "" || data.Reasoning == configs.REASONING_AUTO {
				data.Reasoning = sessReasoning
			}
		}
		if data.Model != sessionconfigbot.DefaultModel && data.Reasoning != "" && data.Reasoning != configs.REASONING_AUTO {
			agent = agents.ModelSet().Registry[data.Model]
			if agent == nil {
				return utilsmsg.New(utilsmsg.NotFound, "model", "%q not registered; use another model", data.Model)
			}
		}
	}
	if agent == nil {
		events <- agentstypes.Event{Type: agentstypes.EventAgentSelect, TaskHash: taskHash, WindowHash: windowHash}

		model := sessionconfigbot.DefaultModel
		if trimModel := strings.TrimSpace(data.Model); trimModel != "" {
			model = trimModel
		}
		skillDesc := ""
		if data.Skill != nil {
			skillDesc = data.Skill.Description
		}
		routed, routedFallbacks, reasoning, err := agentsexecmodel.Route(ctx, model, data.Reasoning, data.Task, skillDesc, sessionID)
		if err != nil {
			return utilsmsg.Trace("agentsexecmodel.Route", "", err)
		}
		agent, fallbacks = routed, routedFallbacks
		if data.Reasoning == "" || data.Reasoning == configs.REASONING_AUTO {
			data.Reasoning = reasoning
		}
	}
	agentName := strings.TrimSpace(agent.Name())
	if IsSchedule(ctx) {
		agentName = ScheduleLabel(DCPushPrefix(ctx))
	}
	agentResult := agentstypes.Event{
		Type:       agentstypes.EventAgentResult,
		Text:       agentName,
		Model:      agent.Name(),
		Reasoning:  ResolveReasoning(sessionID, data.Reasoning).String(),
		TaskHash:   taskHash,
		WindowHash: windowHash,
	}
	events <- agentResult
	sessionlog.Record(sessionID, agentResult)

	data.Agent = agent
	data.FallbackAgents = fallbacks
	session, err := session(ctx, data)
	if err != nil {
		return utilsmsg.Trace("GetSession", "", err)
	}
	return Execute(ctx, data, session, events, data.AllowAll)
}
