package exec

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func CallExternal(ctx context.Context, sessionID, agent, prompt string, events chan<- agentTypes.Event) error {
	label := "external:" + agent

	if !slices.Contains(external.Agents(), agent) {
		msg := fmt.Sprintf("外部 agent %s 未宣告（請設定 EXTERNAL_%s=true）。", agent, strings.ToUpper(agent))
		events <- agentTypes.Event{Type: agentTypes.EventText, Text: msg}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	if err := external.Check(agent); err != nil {
		msg := fmt.Sprintf("外部 agent %s 不可用：%s", agent, err.Error())
		events <- agentTypes.Event{Type: agentTypes.EventText, Text: msg}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	out, err := external.Run(ctx, agent, prompt)
	if err != nil {
		msg := fmt.Sprintf("外部呼叫失敗（%s）：%s", agent, err.Error())
		events <- agentTypes.Event{Type: agentTypes.EventText, Text: msg}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
		return nil
	}

	events <- agentTypes.Event{Type: agentTypes.EventText, Text: out}
	if sessionID != "" {
		writeSessionHistEntry(sessionID, agentTypes.Message{
			Role:    "assistant",
			Content: out,
		})
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: label}
	return nil
}
