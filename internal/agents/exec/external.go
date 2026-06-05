package exec

import (
	"context"
	"fmt"
	"slices"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func CallExternal(ctx context.Context, sessionID, agent, prompt string, readOnly bool, events chan<- agentTypes.Event) error {
	label := "external:" + agent

	if !slices.Contains(external.Agents(), agent) {
		sendText(events, fmt.Sprintf("external agent %s unavailable (not found).", agent))
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	if err := external.Check(agent); err != nil {
		sendText(events, fmt.Sprintf("external agent %s unavailable: %s", agent, err.Error()))
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	out, err := external.Call(ctx, agent, prompt, readOnly)
	if err != nil {
		sendText(events, fmt.Sprintf("failed to call external (%s): %s", agent, err.Error()))
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	sendText(events, out)
	if sessionID != "" {
		writeSessionHistEntry(ctx, sessionID, agentTypes.Message{
			Role:    "assistant",
			Content: out,
		})
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
	return nil
}
