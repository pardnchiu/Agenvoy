package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	sessionconfigbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var channLen = make(chan struct{}, min(configs.MAX_CONCURRENT_TOOLS, configs.MAX_CONCURRENT_SUBAGENTS))

type backgroundKey struct{}

func WithBackground(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundKey{}, true)
}

func isBackground(ctx context.Context) bool {
	v, _ := ctx.Value(backgroundKey{}).(bool)
	return v
}

func Exec(ctx context.Context, task, sessionID, modelName, reasoning, systemPrompt string, excludedTools []string, ignoreHistory, topLevel bool) (string, error) {
	// * step 1: model exist
	registry := agents.ModelSet()
	if len(registry.Registry) == 0 {
		return "", utilsmsg.New(utilsmsg.Unavailable, "subagent model", "none registered")
	}

	// * step 2: concurrent limit
	// * prepare channel to limit concurrent tasks, if full, then retry with a new subagent
	if !isBackground(ctx) {
		select {
		case channLen <- struct{}{}:
			defer func() { <-channLen }()
		case <-ctx.Done():
			return "", utilsmsg.New(utilsmsg.Limit, "subagent slots", "full; call a new subagent to retry this task: %w", ctx.Err())
		}
	}

	// * step 3: session validation and history
	sessID, err := ensureSession(sessionID)
	if err != nil {
		return "", utilsmsg.Trace("subagent session", "", err)
	}
	// * if subagent call with temp-, call it as a new session
	if strings.HasPrefix(sessID, "temp-") {
		ignoreHistory = true
	}

	trimTask := strings.TrimSpace(task)

	// * step 4: working directory
	workDir, ok := ctx.Value(exec.ParentWorkDirKey{}).(string)
	if !ok || workDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			workDir = cwd
		} else if home, err := os.UserHomeDir(); err == nil {
			workDir = home
		} else {
			return "", utilsmsg.New(utilsmsg.Internal, "work directory", "cwd and home are both unreadable")
		}
	}

	// * step 5: allowAll flag
	allowAll, ok := ctx.Value(exec.AllowAllCtxKey{}).(bool)
	if !ok {
		allowAll = true
	}

	// * step 6: excluded tools
	// * collection-only, no nesting, no writes, no deliverable renderers
	baseExclude := []string{
		"subagents",
		"edit_file",
		"edit_tool",
		"test_tool",
		"edit_skill",
		"write_result",
		"html_template",
		"generate*",
		"open_file",
		"schedules",
		"store_secret",
		"pkg_manage",
	}
	if topLevel {
		baseExclude = []string{
			"edit_tool",
			"test_tool",
			"edit_skill",
			"schedules",
			"store_secret",
			"pkg_manage",
		}
	}
	toolExclude := append(append(baseExclude, tools.TUIOnlyTools...), excludedTools...)

	// * step 7: create task
	trimSystemPrompt := strings.TrimSpace(systemPrompt)
	charter := ""
	if !topLevel {
		if trimSystemPrompt != "" {
			trimSystemPrompt = "---\n\n" + trimSystemPrompt
		}
		charter = strings.TrimSpace(strings.ReplaceAll(configs.SubagentPrompt, "{{.SystemPrompt}}", trimSystemPrompt))
	}
	hash := go_pkg_utils.UUID()
	hashPath := filesystem.PendingMetaPath(sessID, hash)
	taskMeta := exec.ExecuteMeta{
		Model:             modelName,
		WorkDir:           workDir,
		SessionID:         sessID,
		TaskHash:          hash,
		KeepPending:       true,
		IgnoreHistory:     ignoreHistory,
		Task:              trimTask,
		Input:             trimTask,
		ExcludeTools:      toolExclude,
		ExcludeSkills:     tools.TUIOnlySkills,
		ExtraSystemPrompt: charter,
		Reasoning:         reasoning,
		AllowAll:          allowAll,
	}

	// * step 8: execute with timeout
	parentSessID := agentstypes.DeliverToFrom(ctx)
	if parentSessID == "" {
		parentSessID = agentstypes.SessionIDFrom(ctx)
	}
	pubsub.Pub(configs.DAEMON_LOG_CHANNEL, agentstypes.Event{
		Type:   agentstypes.EventDaemonLog,
		Source: slog.LevelDebug.String(),
		Text:   "subagent started session=" + sessID + " parent=" + parentSessID,
	})
	timeout := time.Duration(configs.MAX_SUBAGENT_TIMEOUT_MIN) * time.Minute
	if isBackground(ctx) {
		timeout = time.Duration(configs.MAX_BACKGROUND_SUBAGENT_TIMEOUT_MIN) * time.Minute
	}
	subCtx, cancel := context.WithTimeout(agentstypes.WithDeliverTo(ctx, parentSessID), timeout)
	defer cancel()

	parentEvents, ok := ctx.Value(exec.ParentEventsKey{}).(chan<- agentstypes.Event)
	if !ok || isBackground(ctx) {
		parentEvents = nil
	}

	// * step 9: execute in goroutine
	events := make(chan agentstypes.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		defer close(events)
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[Internal] subagent: panic recovered",
					slog.String("session", sessID),
					slog.Any("panic", r))
				errCh <- utilsmsg.New(utilsmsg.Internal, "subagent", "panic: %v", r)
			}
		}()
		errCh <- exec.Run(subCtx, taskMeta, events, true)
	}()

	// * step 10: collect events
	displayName, _ := sessionconfigbot.Get(sessID)
	if displayName == "" || displayName == sessID {
		for _, prefix := range []string{"temp-", "cli-", "dc-", "tg-"} {
			if rest, ok := strings.CutPrefix(sessID, prefix); ok {
				displayName = prefix + rest[:min(len(rest), 8)]
				break
			}
		}
	}

	var (
		sb        strings.Builder
		usage     go_llm_router.Usage
		agentName = modelName
	)
	for ev := range events {
		pubsub.Pub(sessID, ev)
		sendEvent(parentEvents, displayName, ev)

		switch ev.Type {
		case agentstypes.EventAgentResult:
			if ev.Model != "" {
				agentName = ev.Model
			}
		case agentstypes.EventText:
			if ev.Text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(ev.Text)
		case agentstypes.EventDone:
			if ev.Usage != nil {
				usage.Input += ev.Usage.Input
				usage.Output += ev.Usage.Output
				usage.CacheCreate += ev.Usage.CacheCreate
				usage.CacheRead += ev.Usage.CacheRead
			}
		case agentstypes.EventError:
			if ev.Err != nil {
				slog.Debug("subagent event error",
					slog.String("session", sessID),
					slog.String("error", ev.Err.Error()))
			}
		}
	}

	sendEvent(parentEvents, displayName, agentstypes.Event{
		Type:     agentstypes.EventToolResult,
		ToolName: "subagents",
	})

	formatUsage := func(usage go_llm_router.Usage) string {
		return fmt.Sprintf("usage: in=%d out=%d cached=%d write=%d", usage.Input+usage.CacheRead+usage.CacheCreate, usage.Output, usage.CacheRead, usage.CacheCreate)
	}
	usageText := formatUsage(usage)
	retryMessage := ""
	if ctx.Err() == nil {
		retryMessage = fmt.Sprintf(" Call a new subagent to retry only this task, with a model other than %s.", agentName)
	}

	execErr := <-errCh
	result := strings.TrimSpace(sb.String())
	if execErr == nil && result == "" {
		if info, ok := interactive.LoadPendingInfo(sessID, hash); ok && info.HasQuestions {
			answered, resumeUsage := waitResume(subCtx, sessID, parentEvents, displayName)
			if answered != "" {
				result = answered
				usage.Input += resumeUsage.Input
				usage.Output += resumeUsage.Output
				usage.CacheCreate += resumeUsage.CacheCreate
				usage.CacheRead += resumeUsage.CacheRead
				usageText = formatUsage(usage)
			}
		}
	}

	if execErr == nil && result != "" {
		interactive.CleanupPending(sessID, hash)
		return fmt.Sprintf("[subagent · %s · session=%s · %s]\n%s", agentName, sessID, usageText, result), nil
	}

	interactive.CompactPending(sessID, hash)
	partialHint := ""
	if go_pkg_filesystem_reader.Exists(hashPath) {
		partialHint = fmt.Sprintf(" Partial results are saved at %s; have the new subagent read_files it first and continue from there.", hashPath)
	}

	if execErr != nil {
		if result != "" {
			slog.Debug("subagent partial output discarded",
				slog.String("session", sessID),
				slog.String("model", agentName),
				slog.String("output", go_pkg_utils.TruncateString(result, 2048)))
		}
		return "", utilsmsg.New(utilsmsg.Upstream, "subagent "+agentName, "failed: %w.%s%s", execErr, retryMessage, partialHint)
	}
	return "", utilsmsg.New(utilsmsg.Upstream, "subagent "+agentName, "returned no text (%s).%s%s", usageText, retryMessage, partialHint)
}
