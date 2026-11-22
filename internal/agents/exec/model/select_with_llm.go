package model

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/retryHandler"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

func selectWithLLM(ctx context.Context, meta SelectMeta, selectionPrompt string, deadMap map[string]bool) ([]string, string) {
	// * step 1: check cooling down
	agent := retryHandler.Check(meta.Agent, meta.ModelSet)
	if agent == nil {
		return nil, ""
	}

	// * step 2: prepare messages
	orderRaw, err := json.Marshal(meta.ModelSet.Order)
	if err != nil {
		return nil, ""
	}

	workKinds := make([]string, 0, len(config.WorkKinds))
	for _, k := range config.WorkKinds {
		workKinds = append(workKinds, "`"+k.Key+"`")
	}
	systemPrompt := strings.NewReplacer(
		"{{.WorkKinds}}", strings.Join(workKinds, ", "),
		"{{.ModelSelection}}", strings.TrimSpace(selectionPrompt),
	).Replace(strings.TrimSpace(configs.AgentSelector))

	skillLine := ""
	if meta.SkillDescription != "" {
		skillLine = "Skill: " + meta.SkillDescription
	}
	trimTask := strings.TrimSpace(strings.NewReplacer(
		"{{.Agents}}", string(orderRaw),
		"{{.Request}}", strings.TrimSpace(meta.Task),
		"{{.Skill}}", skillLine,
	).Replace(configs.AgentSelectorRequest))

	messages := []go_llm_router.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: trimTask},
	}

	// * step 3: send request
	dispatchCtx := agenttypes.WithSessionID(ctx, meta.SessionID)
	for range len(meta.ModelSet.Order) {
		if ctx.Err() != nil {
			return nil, ""
		}

		sendCtx, cancel := context.WithTimeout(dispatchCtx, configs.TIMEOUT_DISPATCH_CALL)
		resp, code, err := agent.Send(sendCtx, messages, nil, go_llm_router.ReasoningNone, fast.Mode(ctx))
		cancel()
		if err == nil {
			retryHandler.Clear(agent.Name())
			if resp == nil || len(resp.Choices) == 0 {
				return nil, ""
			}

			reply, ok := resp.Choices[0].Message.Content.(string)
			if !ok {
				return nil, ""
			}
			return parseDispatchReply(reply)
		}

		deadMap[agent.Name()] = true
		rateLimited := code == 429
		if rateLimited {
			// * if rate limit then mark as cooling down
			retryHandler.Register(agent.Name())
		}

		// * use fallback in the list, item by item
		next := retryHandler.Check(nil, meta.ModelSet)
		if ctx.Err() == nil && !rateLimited {
			slog.Debug("[Unavailable] dispatcher: routing failed, switching to next",
				slog.String("name", agent.Name()),
				slog.String("error", err.Error()))
		}

		// * if is the last and dead then return empty
		if next == nil || deadMap[next.Name()] {
			return nil, ""
		}
		agent = next
	}
	return nil, ""
}

func parseDispatchReply(reply string) ([]string, string) {
	modelNames := make([]string, 0, 8)
	reasoning := ""
	for line := range strings.SplitSeq(reply, "\n") {
		line = strings.Trim(strings.TrimSpace(line), "\"'` ")
		if line == "" {
			continue
		}
		if reasoning == "" {
			key := strings.ToLower(line)
			if _, ok := config.WorkTiers(key); ok {
				reasoning = key
				continue
			}
		}
		for e := range strings.SplitSeq(line, ",") {
			if e = strings.Trim(strings.TrimSpace(e), "\"'`"); e != "" {
				modelNames = append(modelNames, e)
			}
		}
	}
	// * task type (code / research / ...) to reasoning
	reasoning = config.WorkReasoning(reasoning)
	return modelNames, reasoning
}
