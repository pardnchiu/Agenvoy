package chatCompletions

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	internalutils "github.com/pardnchiu/agenvoy/internal/utils"
)

var (
	codeBlovkRegex    = regexp.MustCompile("(?s)```[^\n]*\n.*?```")
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
)

func stream(c *gin.Context, id string, created int64, model string, events <-chan agenttypes.Event) {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	writeChunk := func(choices []gin.H, extra gin.H) bool {
		chunk := gin.H{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": choices,
		}
		maps.Copy(chunk, extra)
		raw, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !writeChunk([]gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": ""}, "finish_reason": nil}}, nil) {
		return
	}

	ctx := c.Request.Context()
	var usage go_llm_router.Usage
	var streamErr error
	var clientCalls []go_llm_router.ToolCall

	emitContent := func(str string) bool {
		if str == "" {
			return true
		}
		return writeChunk([]gin.H{{"index": 0, "delta": gin.H{"content": normalizeMarkdown(str)}, "finish_reason": nil}}, nil)
	}
	emitReasoningLine := func(line string) bool {
		if line == "" {
			return true
		}
		return writeChunk([]gin.H{{"index": 0, "delta": gin.H{"reasoning_content": line + "\n\n"}, "finish_reason": nil}}, nil)
	}

	var lastReasoning string
	emitDedup := func(line string) bool {
		if line == "" || line == lastReasoning {
			return true
		}
		lastReasoning = line
		return emitReasoningLine(line)
	}

	for ev := range events {
		select {
		case <-ctx.Done():
			return
		default:
		}
		switch ev.Type {
		case agenttypes.EventAgentResult:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitDedup("▸ "+t) {
				return
			}
		case agenttypes.EventSkillResult:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitDedup("▸ skill: "+t) {
				return
			}
		case agenttypes.EventToolCall:
			if internalutils.HideToolEvent(ev.ToolName, ev.ToolArgs) {
				break
			}
			line := internalutils.FormatToolEvent(ev.ToolName, ev.ToolArgs)
			if line == "" {
				break
			}
			if !emitDedup("▸ " + line) {
				return
			}
		case agenttypes.EventToolSkipped:
			if ev.ToolName != "" && !emitDedup("▸ skipped: "+ev.ToolName) {
				return
			}
		case agenttypes.EventExecError:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitReasoningLine("⚠ "+t) {
				return
			}
		case agenttypes.EventReasoning:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitReasoningLine(t) {
				return
			}
		case agenttypes.EventTextDelta:
			if !emitContent(ev.Text) {
				return
			}
		case agenttypes.EventClientToolCall:
			clientCalls = append(clientCalls, ev.ClientToolCalls...)
		case agenttypes.EventDone:
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case agenttypes.EventError:
			if ev.Err != nil {
				streamErr = ev.Err
			}
		}
	}

	switch {
	case streamErr != nil:
		errChunk := []gin.H{{"index": 0, "delta": gin.H{"content": "\n[error] " + streamErr.Error()}, "finish_reason": "stop"}}
		writeChunk(errChunk, nil)
	case len(clientCalls) > 0:
		writeChunk([]gin.H{{"index": 0, "delta": gin.H{"tool_calls": toolCallPayload(clientCalls)}}}, nil)
		writeChunk([]gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "tool_calls"}}, nil)
	default:
		writeChunk([]gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}}, nil)
	}

	promptTokens := usage.Input + usage.CacheRead
	writeChunk([]gin.H{}, gin.H{"usage": gin.H{
		"prompt_tokens":     promptTokens,
		"completion_tokens": usage.Output,
		"total_tokens":      promptTokens + usage.Output,
		"prompt_tokens_details": gin.H{
			"cached_tokens": usage.CacheRead,
		},
	}})

	if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err == nil {
		flusher.Flush()
	}
}

func normalizeMarkdown(str string) string {
	var blocks []string
	str = codeBlovkRegex.ReplaceAllStringFunc(str, func(m string) string {
		blocks = append(blocks, m)
		return fmt.Sprintf("\x00CB%d\x00", len(blocks)-1)
	})

	str = strings.ReplaceAll(str, "\n", "\n\n")
	str = multiNewlineRegex.ReplaceAllString(str, "\n\n")

	for i, b := range blocks {
		str = strings.Replace(str, fmt.Sprintf("\x00CB%d\x00", i), b, 1)
	}
	return str
}
