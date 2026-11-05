package subagent

import (
	"fmt"
	"io"
	"time"

	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func writeBackgroundLog(w io.Writer, ev agentstypes.Event) {
	if w == nil {
		return
	}
	now := time.Now().Format("15:04:05")
	switch ev.Type {
	case agentstypes.EventAgentResult:
		fmt.Fprintf(w, "%s model: %s\n", now, ev.Model)
	case agentstypes.EventToolCall:
		fmt.Fprintf(w, "%s call: %s %s\n", now, ev.ToolName, ev.ToolArgs)
	case agentstypes.EventToolResult:
		fmt.Fprintf(w, "%s result: %s\n%s\n", now, ev.ToolName, ev.Result)
	case agentstypes.EventText:
		if ev.Text != "" {
			fmt.Fprintf(w, "%s text:\n%s\n", now, ev.Text)
		}
	case agentstypes.EventError, agentstypes.EventExecError:
		if ev.Err != nil {
			fmt.Fprintf(w, "%s error: %s\n", now, ev.Err.Error())
		} else if ev.Text != "" {
			fmt.Fprintf(w, "%s error: %s\n", now, ev.Text)
		}
	}
}
