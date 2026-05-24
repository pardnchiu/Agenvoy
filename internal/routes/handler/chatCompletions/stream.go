package chatCompletions

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	internalUtils "github.com/pardnchiu/agenvoy/internal/utils"
)

var (
	codeBlovkRegex    = regexp.MustCompile("(?s)```[^\n]*\n.*?```")
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
)

func stream(c *gin.Context, id string, created int64, model string, events <-chan agentTypes.Event) {
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
		data, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !writeChunk([]gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": ""}, "finish_reason": nil}}, nil) {
		return
	}

	ctx := c.Request.Context()
	var usage agentTypes.Usage
	var streamErr error

	emitContent := func(text string) bool {
		if text == "" {
			return true
		}
		return writeChunk([]gin.H{{"index": 0, "delta": gin.H{"content": normalizeMarkdown(text)}, "finish_reason": nil}}, nil)
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
	formatToolLine := func(name, args string) string {
		arg := internalUtils.FormatTool(name, args)
		if arg == "" {
			return name
		}
		const max = 80
		if r := []rune(arg); len(r) > max {
			arg = string(r[:max]) + "…"
		}
		return name + "  " + arg
	}

	for ev := range events {
		select {
		case <-ctx.Done():
			return
		default:
		}
		switch ev.Type {
		case agentTypes.EventAgentResult:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitDedup("▸ "+t) {
				return
			}
		case agentTypes.EventSkillResult:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitDedup("▸ skill: "+t) {
				return
			}
		case agentTypes.EventToolCall:
			if ev.ToolName == "" || ev.ToolName == "ask_user" || ev.ToolName == "store_secret" {
				break
			}
			if !emitDedup("▸ " + formatToolLine(ev.ToolName, ev.ToolArgs)) {
				return
			}
		case agentTypes.EventToolSkipped:
			if ev.ToolName != "" && !emitDedup("▸ skipped: "+ev.ToolName) {
				return
			}
		case agentTypes.EventExecError:
			if t := strings.TrimSpace(ev.Text); t != "" && !emitReasoningLine("⚠ "+t) {
				return
			}
		case agentTypes.EventText:
			if !emitContent(ev.Text + "\n") {
				return
			}
		case agentTypes.EventDone:
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case agentTypes.EventError:
			if ev.Err != nil {
				streamErr = ev.Err
			}
		}
	}

	if streamErr != nil {
		errChunk := []gin.H{{"index": 0, "delta": gin.H{"content": "\n[error] " + streamErr.Error()}, "finish_reason": "stop"}}
		writeChunk(errChunk, nil)
	} else {
		writeChunk([]gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}}, nil)
	}

	writeChunk([]gin.H{}, gin.H{"usage": gin.H{
		"prompt_tokens":     usage.Input,
		"completion_tokens": usage.Output,
		"total_tokens":      usage.Input + usage.Output,
	}})

	if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err == nil {
		flusher.Flush()
	}
}

func normalizeMarkdown(text string) string {
	var blocks []string
	text = codeBlovkRegex.ReplaceAllStringFunc(text, func(m string) string {
		blocks = append(blocks, m)
		return fmt.Sprintf("\x00CB%d\x00", len(blocks)-1)
	})

	text = strings.ReplaceAll(text, "\n", "\n\n")
	text = multiNewlineRegex.ReplaceAllString(text, "\n\n")

	for i, b := range blocks {
		text = strings.Replace(text, fmt.Sprintf("\x00CB%d\x00", i), b, 1)
	}
	return text
}
