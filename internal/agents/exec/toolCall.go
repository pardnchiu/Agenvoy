package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agentsexecallowtool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/memory"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/toolcache"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const foregroundOrigin = "cli-"

func askUserInBackground(sessionID, origin, deliverTo, taskHash, rawArgs string, toolResults []interactive.ToolResult, files []string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("askUserInBackground panic recovered",
				slog.String("session", sessionID),
				slog.Any("panic", r))
		}
	}()
	var params struct {
		Questions []runtime.Question `json:"questions"`
		State     struct {
			Objective string   `json:"objective"`
			Completed []string `json:"completed"`
			NextSteps []string `json:"next_steps"`
		} `json:"state"`
	}
	if err := json.Unmarshal([]byte(rawArgs), &params); err != nil {
		slog.Debug("json Unmarshal",
			slog.String("error", err.Error()))
		return
	}
	if len(params.Questions) == 0 {
		slog.Debug("ask user no questions")
		return
	}

	hash := interactive.SaveAndEnqueueAskUser(sessionID, origin, deliverTo, params.Questions, params.State.Objective, params.State.Completed, params.State.NextSteps, toolResults, taskHash, files)
	event := agenttypes.Event{Type: agenttypes.EventPending, Text: hash, TaskHash: hash, PendingSession: sessionID}
	sessionlog.Record(sessionID, event)
	pubsub.Pub(sessionID, event)
	if deliverTo != "" && deliverTo != sessionID {
		pubsub.Pub(deliverTo, event)
	}
}

func originFor(ctx context.Context, sessionID string) string {
	if origin := agenttypes.OriginFrom(ctx); origin != "" {
		return origin
	}
	return runtime.OriginOf(sessionID)
}

func deliverFor(ctx context.Context, sessionID string) string {
	if id := agenttypes.DeliverToFrom(ctx); id != "" {
		return id
	}
	return sessionID
}

var ErrAskUserInterrupted = errors.New("ask user interrupted")

func toolResults(session *agenttypes.Session) []interactive.ToolResult {
	nameByID := make(map[string]string)
	argsByID := make(map[string]string)
	for _, msg := range session.ToolHistories {
		for _, tc := range msg.ToolCalls {
			nameByID[tc.ID] = tc.Function.Name
			argsByID[tc.ID] = tc.Function.Arguments
		}
	}

	var results []interactive.ToolResult
	for _, msg := range session.ToolResults {
		content, _ := msg.Content.(string)
		results = append(results, interactive.ToolResult{
			Name:   nameByID[msg.ToolCallID],
			ID:     msg.ToolCallID,
			Args:   argsByID[msg.ToolCallID],
			Result: content,
		})
	}
	return results
}

const (
	slotReady          = 0
	slotCached         = 1
	slotSkipped        = 2
	slotSchemaMarked   = 3
	slotValidateFailed = 4
	slotDispatched     = 5
)

type toolSlot struct {
	idx  int
	id   string
	name string
	args string
	hash string

	state     int
	preMsg    string
	imageURLs []string

	result     string
	execErr    string
	execErrVal error
}

func toolNeedsConfirmation(exec *tooltypes.Executor, toolName, toolArgs string, turnAllowAll bool) bool {
	if toolName == "read_files" && isSensitiveReadFile(toolArgs) {
		return true
	}
	if turnAllowAll {
		return false
	}
	if isDestructiveMode(toolArgs) {
		return true
	}
	if toolName == "open_file" && isOpenFileURL(toolArgs) {
		return true
	}
	if toolregister.IsReadOnly(toolName) {
		return false
	}
	if isReadOnlyMode(toolArgs) {
		return false
	}
	if toolName == "http_request" && isGet(toolArgs) {
		return false
	}
	if toolName == "run_command" && isReadOnlyRunCommand(toolArgs) {
		return false
	}
	return !agentsexecallowtool.Match(exec.WorkDir, toolName, toolArgs)
}

var readOnlyModes = map[string]bool{
	"list":   true,
	"read":   true,
	"search": true,
}

var destructiveModes = map[string]bool{
	"remove":  true,
	"restore": true,
}

func isDestructiveMode(toolArgs string) bool {
	var p struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil {
		return false
	}
	return destructiveModes[strings.TrimSpace(p.Mode)]
}

func isOpenFileURL(toolArgs string) bool {
	var p struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil {
		return false
	}
	return tools.IsOpenURL(p.Path)
}

func isReadOnlyMode(toolArgs string) bool {
	var p struct {
		Mode string `json:"mode"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil {
		return false
	}
	return readOnlyModes[strings.TrimSpace(p.Mode)]
}

func hasDangerousGitFlag(args []string) bool {
	for _, a := range args {
		switch {
		case a == "-o", a == "--output", a == "--output-directory":
			return true
		case strings.HasPrefix(a, "--output=") || strings.HasPrefix(a, "--output-directory="):
			return true
		case strings.HasPrefix(a, "-o") && a != "-o":
			return true
		}
	}
	return false
}

func stripSafeGitGlobalFlags(argv []string) []string {
	i := 1
	for i < len(argv) && strings.HasPrefix(argv[i], "-") {
		if argv[i] == "-c" || strings.HasPrefix(argv[i], "-c=") {
			break
		}
		if !strings.Contains(argv[i], "=") && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
			i += 2
		} else {
			i++
		}
	}
	return append([]string{argv[0]}, argv[i:]...)
}

func readOnlyCandidates(bin string, argv []string) []string {
	list := make([]string, 0, 3)
	if len(argv) > 2 {
		list = append(list, bin+" "+argv[1]+" "+argv[2])
	}
	if len(argv) > 1 {
		list = append(list, bin+" "+argv[1])
	}
	return append(list, bin)
}

func isReadOnlyRunCommand(toolArgs string) bool {
	var p struct {
		Argv    []string `json:"argv"`
		Network bool     `json:"network"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil || len(p.Argv) == 0 || p.Network {
		return false
	}
	argv := p.Argv
	bin := filepath.Base(argv[0])
	if bin == "git" {
		argv = stripSafeGitGlobalFlags(argv)
	}

	matched := false
	for _, candidate := range readOnlyCandidates(bin, argv) {
		if slices.Contains(filesystem.ReadOnlyCommand, candidate) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	if slices.ContainsFunc(argv[1:], boundary.IsSensitivePath) {
		return false
	}
	if bin == "git" {
		return !hasDangerousGitFlag(argv[2:])
	}
	return true
}

func invalidateReadFileCache(alreadyCall map[string]string, writeArgsJSON string) {
	var p struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(writeArgsJSON), &p) != nil || p.Path == "" {
		return
	}
	for key := range alreadyCall {
		if strings.HasPrefix(key, "read_files|") && strings.Contains(key, p.Path) {
			delete(alreadyCall, key)
		}
	}
}

var isWriteLikeTool = map[string]bool{
	"edit_file":    true,
	"edit_skill":   true,
	"edit_tool":    true,
	"write_result": true,
}

func truncateWriteArgs(argsJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(argsJSON), &m) != nil {
		return argsJSON
	}

	kept := map[string]any{}
	for key, value := range m {
		if key == "content" || key == "targets" {
			continue
		}
		kept[key] = value
	}
	if targets, ok := m["targets"].([]any); ok {
		kept["edits"] = len(targets)
	}
	if content, ok := m["content"].(string); ok {
		kept["wrote_bytes"] = len(content)
	}

	out, err := json.Marshal(kept)
	if err != nil {
		return argsJSON
	}
	return string(out)
}

func isSensitiveReadFile(argsJSON string) bool {
	var p struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if json.Unmarshal([]byte(argsJSON), &p) != nil {
		return false
	}
	for _, f := range p.Files {
		if f.Path != "" && boundary.IsSensitivePath(f.Path) {
			return true
		}
	}
	return false
}

func isGet(argsJSON string) bool {
	var p struct {
		Method string `json:"method"`
	}
	if json.Unmarshal([]byte(argsJSON), &p) != nil {
		return false
	}
	return p.Method == "" || strings.EqualFold(p.Method, "GET")
}

func toolCall(ctx context.Context, exec *tooltypes.Executor, choice go_llm_router.OutputChoices, sessionData *agenttypes.Session, events chan<- agenttypes.Event, allowAll bool, alreadyCall map[string]string, turnAllowAll *bool) (*agenttypes.Session, map[string]string, error) {
	sessionData.ToolHistories = append(sessionData.ToolHistories, choice.Message)

	calls := choice.Message.ToolCalls
	slots := make([]toolSlot, len(calls))
	activatedInBatch := make(map[string]bool)

	for i, tool := range calls {
		toolID := strings.TrimSpace(tool.ID)
		toolArg := strings.TrimSpace(tool.Function.Arguments)
		toolName := strings.TrimSpace(tool.Function.Name)
		if idx := strings.Index(toolName, "<|"); idx != -1 {
			toolName = toolName[:idx]
		}
		wrapped := toolName == "run_tool"
		if wrapped {
			name, args, err := unwrapRunTool(exec, toolArg)
			if err != nil {
				slots[i] = toolSlot{
					idx:    i,
					id:     toolID,
					name:   toolName,
					args:   toolArg,
					state:  slotValidateFailed,
					preMsg: fmt.Sprintf("tool=run_tool failed: %s", err.Error()),
				}
				continue
			}
			toolName, toolArg = name, args
		}
		hashArg := toolArg
		var argMap map[string]any
		if json.Unmarshal([]byte(toolArg), &argMap) == nil {
			if normalized, err := json.Marshal(argMap); err == nil {
				hashArg = string(normalized)
			}
		}
		hash := fmt.Sprintf("%v|%v", toolName, hashArg)

		slots[i] = toolSlot{
			idx:   i,
			id:    toolID,
			name:  toolName,
			args:  toolArg,
			hash:  hash,
			state: slotReady,
		}

		interactive.RecordToolAttempt(exec.SessionID, exec.PendingTask, interactive.ToolAttempt{
			Name: toolName,
			ID:   toolID,
			Args: toolArg,
		})

		if !wrapped && !slices.ContainsFunc(exec.Tools, func(t go_llm_router.Tool) bool { return t.Function.Name == toolName }) {
			reason := ""
			switch {
			case exec.ExcludeTools[toolName]:
				reason = fmt.Sprintf("tool=%s failed: %s is not available in this session", toolName, toolName)
			case slices.ContainsFunc(exec.AllTools, func(t go_llm_router.Tool) bool { return t.Function.Name == toolName }):
				reason = fmt.Sprintf("tool=%s failed: %s is not callable directly. Fetch its schema with find_tools, then call run_tool(name=%s, args=...).", toolName, toolName, toolName)
			}
			if reason != "" {
				slots[i].state = slotValidateFailed
				slots[i].preMsg = reason
				continue
			}
		}

		if cached, ok := alreadyCall[hash]; ok && cached != "" {
			cachedContent := strings.TrimSpace(cached)
			if images, rest := splitImageResult(cached); len(images) > 0 {
				cachedContent = imageLoadedMessage(toolName, len(images), rest)
				slots[i].imageURLs = images
			}
			slots[i].state = slotCached
			slots[i].preMsg = cachedContent
			continue
		}

		if exec.UnmarkedTools[toolName] || activatedInBatch[toolName] {
			schema := ""
			if exec.UnmarkedTools[toolName] {
				activateArgs, _ := json.Marshal(map[string]any{"mode": "search", "names": []string{toolName}})
				out, err := toolregister.Dispatch(ctx, exec, "find_tools", activateArgs)
				if err != nil {
					slog.Warn("tool schema marking failed",
						slog.String("name", toolName),
						slog.String("error", err.Error()))
				}
				schema = out
				delete(exec.UnmarkedTools, toolName)
			}
			activatedInBatch[toolName] = true
			reinvoke := toolName
			if wrapped {
				reinvoke = "run_tool(name=" + toolName + ")"
			}
			slots[i].state = slotSchemaMarked
			slots[i].preMsg = fmt.Sprintf("[%s] tool schema below. Re-invoke %s with the correct arguments — the previous call was made before its schema was fetched.\n%s", toolName, reinvoke, schema)
			continue
		}

		if !toolregister.Exists(toolName) {
			events <- agenttypes.Event{
				Type:     agenttypes.EventToolCall,
				ToolName: toolName,
				ToolArgs: toolArg,
				ToolID:   toolID,
			}
			slots[i].state = slotValidateFailed
			slots[i].preMsg = fmt.Sprintf("tool=%s failed: not exist: %s. Look up available tools with find_tools.", toolName, toolName)
			continue
		}

		if earlyErr := validateToolArgs(exec, toolName, toolArg); earlyErr != "" {
			events <- agenttypes.Event{
				Type:     agenttypes.EventToolCall,
				ToolName: toolName,
				ToolArgs: toolArg,
				ToolID:   toolID,
			}
			content := fmt.Sprintf("tool=%s failed: %s", toolName, earlyErr)
			slots[i].state = slotValidateFailed
			slots[i].preMsg = content
			continue
		}

		restrictedPaths := boundary.Restricted(exec.SessionID, exec.WorkDir, toolName, toolArg)
		restrictedList := append(append([]string{}, restrictedPaths...), interactive.RestrictedPkgManage(toolName, toolArg)...)
		restricted := len(restrictedList) > 0

		if !allowAll && (restricted || toolNeedsConfirmation(exec, toolName, toolArg, *turnAllowAll)) {
			proceed := true
			approved := false
			verified := false
			reason := ""
			origin := originFor(ctx, sessionData.ID)
			if runtime.HasListener(ctx, origin) {
				askCtx, cancelAsk := context.WithCancel(ctx)
				if agenttypes.OriginFrom(ctx) != foregroundOrigin {
					askCtx, cancelAsk = context.WithTimeout(ctx, configs.CONFIRM_TIMEOUT)
				}
				reply, err := runtime.Ask(askCtx, runtime.Request{
					Kind:       runtime.KindToolConfirm,
					SessionID:  sessionData.ID,
					Origin:     origin,
					DeliverTo:  deliverFor(ctx, sessionData.ID),
					ToolName:   toolName,
					ToolArgs:   toolArg,
					Restricted: restrictedList,
				})
				cancelAsk()
				if errors.Is(err, context.DeadlineExceeded) {
					events <- agenttypes.Event{
						Type:     agenttypes.EventToolSkipped,
						ToolName: toolName,
						ToolArgs: toolArg,
						ToolID:   toolID,
						Text:     "no answer within " + configs.CONFIRM_TIMEOUT.String() + "; task kept as pending",
					}
					if exec.CancelExecution != nil {
						exec.CancelExecution()
					}
					return sessionData, alreadyCall, utilsmsg.New(utilsmsg.Timeout, "tool confirmation", "timed out after %s; resume from pending to continue", configs.CONFIRM_TIMEOUT)
				}
				if errors.Is(err, context.Canceled) {
					if errors.Is(context.Cause(ctx), runtime.ErrUserCanceled) {
						interactive.DeletePending(sessionData.ID, exec.PendingTask)
					}
					return sessionData, alreadyCall, err
				}
				if err == nil && reply.Error != nil {
					events <- agenttypes.Event{
						Type:     agenttypes.EventToolSkipped,
						ToolName: toolName,
						ToolArgs: toolArg,
						ToolID:   toolID,
						Text:     reply.Error.Error(),
					}
					if errors.Is(reply.Error, runtime.ErrUserCanceled) {
						interactive.DeletePending(sessionData.ID, exec.PendingTask)
					}
					if exec.CancelExecution != nil {
						exec.CancelExecution()
					}
					return sessionData, alreadyCall, reply.Error
				}
				if err != nil {
					proceed = false
				} else {
					proceed = reply.Approve
					approved = reply.Approve
					verified = reply.Verified
					reason = reply.Reason
					if reply.Approve && reply.Remember {
						if _, err = agentsexecallowtool.Toggle(toolName, toolArg, exec.WorkDir); err != nil {
							slog.Warn("appendAllowListRule",
								slog.String("session", sessionData.ID),
								slog.String("error", err.Error()))
						}
					}
					if reply.Approve && reply.AllowTurn {
						*turnAllowAll = true
					}
				}
			}
			if approved && restricted && !verified {
				proceed = false
				approved = false
				reason = fmt.Sprintf("this needs system password verification, which this channel cannot collect: %s", strings.Join(restrictedList, ", "))
			}
			if !proceed {
				message := "The user declined this call. Retrying it, or reissuing it with different arguments, produces the same prompt and the same refusal. Continue without it, or use ask_user to agree on another approach."
				if reason != "" {
					message = fmt.Sprintf("The user declined this call: %s. Retrying it, or reissuing it with different arguments, produces the same prompt and the same refusal. Continue without it, or use ask_user to agree on another approach.", reason)
				}
				events <- agenttypes.Event{
					Type:     agenttypes.EventToolSkipped,
					ToolName: toolName,
					ToolArgs: toolArg,
					ToolID:   toolID,
					Text:     reason,
				}
				slots[i].state = slotSkipped
				slots[i].preMsg = message
				continue
			}
			if approved {
				boundary.Grant(exec.SessionID, restrictedPaths...)
			}
		}

	}

	for i := range slots {
		slot := &slots[i]
		if slot.state == slotReady && slot.name == "ask_user" && agenttypes.OriginFrom(ctx) != foregroundOrigin {
			var imageURLs []string
			for j := range slots {
				cs := &slots[j]
				if cs.state == slotReady || cs.name == "ask_user" {
					continue
				}
				content := cs.preMsg
				msg := go_llm_router.Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: cs.id,
				}
				switch cs.state {
				case slotCached:
					imageURLs = append(imageURLs, cs.imageURLs...)
					sessionData.ToolHistories = append(sessionData.ToolHistories, msg)
				default:
					sessionData.ToolResults = append(sessionData.ToolResults, msg)
					sessionData.ToolHistories = append(sessionData.ToolHistories, msg)
				}
			}
			appendImageMessage(sessionData, imageURLs)

			toolResults := toolResults(sessionData)

			go askUserInBackground(sessionData.ID, originFor(ctx, sessionData.ID), deliverFor(ctx, sessionData.ID), exec.PendingTask, slot.args, toolResults, exec.EditedFiles())
			if exec.CancelExecution != nil {
				exec.CancelExecution()
			}
			return sessionData, alreadyCall, ErrAskUserInterrupted
		}
	}

	var wg sync.WaitGroup
	toolSlots := make(chan struct{}, configs.MAX_CONCURRENT_TOOLS)
	for i := range slots {
		s := &slots[i]
		if s.state != slotReady {
			continue
		}
		if toolregister.IsBackground(s.name) {
			go runToolExec(ctx, exec, s, events)
			s.result = "ok"
			s.state = slotDispatched
			continue
		}
		if toolregister.IsConcurrent(s.name) {
			wg.Add(1)
			go func(s *toolSlot) {
				defer wg.Done()
				toolSlots <- struct{}{}
				defer func() { <-toolSlots }()
				runToolExec(ctx, exec, s, events)
			}(s)
			s.state = slotDispatched
			continue
		}
	}
	for i := range slots {
		s := &slots[i]
		if s.state != slotReady {
			continue
		}
		runToolExec(ctx, exec, s, events)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return sessionData, alreadyCall, err
	}

	var imageURLs []string
	for i := range slots {
		s := &slots[i]
		switch s.state {
		case slotCached:
			imageURLs = append(imageURLs, s.imageURLs...)
			sessionData.ToolHistories = append(sessionData.ToolHistories, go_llm_router.Message{
				Role:       "tool",
				Content:    s.preMsg,
				ToolCallID: s.id,
			})
			continue
		case slotSkipped, slotSchemaMarked, slotValidateFailed:
			msg := go_llm_router.Message{
				Role:       "tool",
				Content:    s.preMsg,
				ToolCallID: s.id,
			}
			sessionData.ToolResults = append(sessionData.ToolResults, msg)
			sessionData.ToolHistories = append(sessionData.ToolHistories, msg)
			continue
		}

		result := s.result
		historyResult := ""
		if s.execErr != "" {
			hint := memory.Search(ctx, s.name, s.execErr, 3)
			if hint != "" {
				result = fmt.Sprintf("tool=%s failed: %s\nrelated_errors: %s", s.name, s.execErr, hint)
			} else {
				result = fmt.Sprintf("tool=%s failed: %s", s.name, s.execErr)
			}
		} else if result == "" || result == "no data" {
			if hint := memory.Search(ctx, s.name, "no data", 3); hint != "" {
				result = hint
			} else {
				result = "no data"
			}
		}

		if s.name == "edit_file" && s.execErr == "" {
			invalidateReadFileCache(alreadyCall, s.args)
		}
		if isWriteLikeTool[s.name] && s.execErr == "" {
			calls[i].Function.Arguments = truncateWriteArgs(calls[i].Function.Arguments)
		}
		if !isWriteLikeTool[s.name] || s.execErr == "" {
			alreadyCall[s.hash] = result
		}
		if images, _ := splitImageResult(result); s.execErr == "" && len(images) == 0 && toolcache.IsCacheable(s.name, s.args) {
			toolcache.Store(exec.SessionID, s.id, s.name, s.args, result)
		}

		events <- agenttypes.Event{
			Type:     agenttypes.EventToolResult,
			ToolName: s.name,
			ToolArgs: s.args,
			ToolID:   s.id,
			Result:   result,
		}

		toolMsgContent := strings.TrimSpace(fmt.Sprintf("[%s] %s", s.name, result))
		if images, rest := splitImageResult(result); len(images) > 0 {
			toolMsgContent = imageLoadedMessage(s.name, len(images), rest)
			imageURLs = append(imageURLs, images...)
		}
		toolMsg := go_llm_router.Message{
			Role:       "tool",
			Content:    toolMsgContent,
			ToolCallID: s.id,
		}
		sessionData.ToolResults = append(sessionData.ToolResults, toolMsg)
		if historyResult != "" {
			sessionData.ToolHistories = append(sessionData.ToolHistories, go_llm_router.Message{
				Role:       "tool",
				Content:    historyResult,
				ToolCallID: s.id,
			})
		} else {
			sessionData.ToolHistories = append(sessionData.ToolHistories, toolMsg)
		}
	}
	appendImageMessage(sessionData, imageURLs)

	return sessionData, alreadyCall, nil
}

func failToolEvent(exec *tooltypes.Executor, s *toolSlot, events chan<- agenttypes.Event, err error) {
	s.execErr = err.Error()
	s.execErrVal = err
	go interactive.AppendToolResult(exec.SessionID, exec.PendingTask, interactive.ToolResult{
		Name:   s.name,
		ID:     s.id,
		Args:   s.args,
		Result: "error: " + err.Error(),
	}, exec.EditedFiles())
	events <- agenttypes.Event{
		Type:     agenttypes.EventToolCallEnd,
		ToolName: s.name,
		ToolID:   s.id,
	}
}

func runToolExec(ctx context.Context, exec *tooltypes.Executor, s *toolSlot, events chan<- agenttypes.Event) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		slog.Error("runToolExec panic recovered",
			slog.String("tool", s.name),
			slog.Any("panic", r))
		failToolEvent(exec, s, events, utilsmsg.New(utilsmsg.Internal, "tool "+s.name, "panicked: %v", r))
	}()
	events <- agenttypes.Event{
		Type:     agenttypes.EventToolCall,
		ToolName: s.name,
		ToolArgs: s.args,
		ToolID:   s.id,
	}
	events <- agenttypes.Event{
		Type:     agenttypes.EventToolCallStart,
		ToolName: s.name,
		ToolID:   s.id,
	}
	var progressMu sync.Mutex
	progressDone := false
	progressCtx := tooltypes.WithProgress(ctx, func(chunk string) {
		progressMu.Lock()
		defer progressMu.Unlock()
		if progressDone {
			return
		}
		events <- agenttypes.Event{
			Type:     agenttypes.EventToolProgress,
			ToolName: s.name,
			ToolID:   s.id,
			Text:     chunk,
		}
	})
	result, err := tools.Execute(progressCtx, exec, s.name, json.RawMessage(s.args))
	progressMu.Lock()
	progressDone = true
	progressMu.Unlock()
	if err != nil {
		failToolEvent(exec, s, events, err)
		return
	}

	if result != "" {
		events <- agenttypes.Event{
			Type:     agenttypes.EventToolCallText,
			ToolName: s.name,
			ToolID:   s.id,
			Text:     result,
		}
	}
	s.result = result
	go interactive.AppendToolResult(exec.SessionID, exec.PendingTask, interactive.ToolResult{
		Name:   s.name,
		ID:     s.id,
		Args:   s.args,
		Result: result,
	}, exec.EditedFiles())
	if s.name == "write_todo" {
		if todos := interactive.LoadTodos(exec.SessionID, exec.PendingTask); len(todos) > 0 {
			events <- agenttypes.Event{
				Type:  agenttypes.EventTodoUpdate,
				Todos: todos,
			}
		}
	}
	events <- agenttypes.Event{
		Type:     agenttypes.EventToolCallEnd,
		ToolName: s.name,
		ToolID:   s.id,
	}
}

func unwrapRunTool(exec *tooltypes.Executor, args string) (string, string, error) {
	var params struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "run_tool args", err)
	}

	name := strings.TrimSpace(params.Name)
	switch {
	case name == "":
		return "", "", utilsmsg.New(utilsmsg.Invalid, "run_tool name", "required")
	case name == "run_tool":
		return "", "", utilsmsg.New(utilsmsg.Invalid, "run_tool name", "run_tool cannot run itself")
	case exec.ExcludeTools[name]:
		return "", "", utilsmsg.New(utilsmsg.Denied, "tool", "%s is not available in this session", name)
	case !slices.ContainsFunc(exec.AllTools, func(t go_llm_router.Tool) bool { return t.Function.Name == name }):
		return "", "", utilsmsg.New(utilsmsg.NotFound, "tool", "not exist: %s; look it up with find_tools", name)
	}

	if params.Args == nil {
		params.Args = map[string]any{}
	}
	raw, err := json.Marshal(params.Args)
	if err != nil {
		return "", "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "run_tool args", err)
	}
	return name, string(raw), nil
}

func validateToolArgs(exec *tooltypes.Executor, toolName, args string) string {
	if exec == nil {
		return ""
	}
	required := requiredFields(exec, toolName)
	if len(required) == 0 {
		return ""
	}

	args = strings.TrimSpace(args)
	var parsed map[string]any
	if args != "" && args != "null" {
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return fmt.Sprintf("invalid JSON for %s: %s. Re-send arguments as a JSON object with required fields: %s",
				toolName, err.Error(), strings.Join(required, ", "))
		}
	}

	var missing []string
	for _, f := range required {
		v, ok := parsed[f]
		if !ok {
			missing = append(missing, f)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("missing required field(s) %s for %s. All required fields: %s",
		strings.Join(missing, ", "), toolName, strings.Join(required, ", "))
}

func requiredFields(exec *tooltypes.Executor, toolName string) []string {
	lookup := func(list []go_llm_router.Tool) []string {
		for _, t := range list {
			if t.Function.Name != toolName {
				continue
			}
			if len(t.Function.Parameters) == 0 {
				return nil
			}
			var schema struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(t.Function.Parameters, &schema); err != nil {
				return nil
			}
			return schema.Required
		}
		return nil
	}
	if r := lookup(exec.AllTools); len(r) > 0 {
		return r
	}
	return lookup(exec.Tools)
}

func splitImageResult(result string) (images []string, rest string) {
	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "data:image/") {
		return []string{trimmed}, ""
	}
	if !strings.HasPrefix(trimmed, "{") {
		return nil, result
	}

	var dic map[string]string
	if json.Unmarshal([]byte(trimmed), &dic) != nil {
		return nil, result
	}

	kept := make(map[string]string, len(dic))
	for _, path := range slices.Sorted(maps.Keys(dic)) {
		if strings.HasPrefix(dic[path], "data:image/") {
			images = append(images, dic[path])
			continue
		}
		kept[path] = dic[path]
	}
	if len(images) == 0 {
		return nil, result
	}
	if len(kept) == 0 {
		return images, ""
	}
	raw, err := json.Marshal(kept)
	if err != nil {
		return images, result
	}
	return images, string(raw)
}

func imageLoadedMessage(toolName string, count int, rest string) string {
	msg := fmt.Sprintf("[%s] %d image(s) loaded", toolName, count)
	if rest != "" {
		msg += " " + rest
	}
	return msg
}

func appendImageMessage(session *agenttypes.Session, dataURLs []string) {
	if len(dataURLs) == 0 {
		return
	}
	parts := make([]go_llm_router.ContentPart, 0, len(dataURLs))
	for _, url := range dataURLs {
		parts = append(parts, go_llm_router.ContentPart{
			Type:     "image_url",
			ImageURL: &go_llm_router.ImageURL{URL: url, Detail: "auto"},
		})
	}
	session.ToolHistories = append(session.ToolHistories, go_llm_router.Message{Role: "user", Content: parts})
}
