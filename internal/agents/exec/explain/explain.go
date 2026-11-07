package explain

import (
	"context"
	"log/slog"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
)

var commandTools = map[string]bool{"run_command": true, "background_command": true}

func Command(ctx context.Context, sessionID, objective, toolName, toolArgs string) string {
	if !commandTools[toolName] {
		return ""
	}
	agent := agents.DispatcherBot()
	if agent == nil {
		return ""
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), configs.CONFIRM_EXPLAIN_TIMEOUT)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()

	prompt := strings.TrimSpace(strings.ReplaceAll(filesystem.ApplyReplyLang(configs.ConfirmExplainPrompt), "{{.ReplyLanguage}}", filesystem.ReplyLangDirective()))
	user := "<request>\n" + go_pkg_utils.TruncateString(strings.TrimSpace(objective), configs.MAX_DECISION_RUNES) + "\n</request>\n<tool>" + toolName + "</tool>\n<args>\n" + toolArgs + "\n</args>"

	start := time.Now()
	resp, _, err := agent.Send(sendCtx, []go_llm_router.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: user},
	}, nil, go_llm_router.ReasoningNone, fast.Mode(ctx))
	elapsed := time.Since(start)
	if err != nil {
		slog.Debug("explain.Command",
			slog.String("session", sessionID),
			slog.String("model", agent.Name()),
			slog.String("error", err.Error()))
		return ""
	}
	if len(resp.Choices) == 0 {
		return ""
	}

	prov, model, _ := strings.Cut(agent.Name(), "@")
	usagelog.Append(sessionID, prov, model, resp.Usage, elapsed, nil)

	content, _ := resp.Choices[0].Message.Content.(string)
	content = strings.Join(strings.Fields(content), " ")
	return go_pkg_utils.TruncateString(content, configs.CONFIRM_EXPLAIN_RUNES)
}
