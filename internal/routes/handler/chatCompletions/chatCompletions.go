package chatCompletions

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pardnchiu/go-pkg/utils"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

var (
	zedWorkDirRegex = regexp.MustCompile("(?m)^- `(/[^`]+)`")
)

type Request struct {
	Model    string               `json:"model"`
	Messages []agentTypes.Message `json:"messages"`
	Stream   bool                 `json:"stream"`
	workDir  string               `json:"-"`
}

func ChatCompletions() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			}})
			return
		}

		workDir := extractWorkDirFromZed(req.Messages)
		if workDir != "" {
			req.workDir = workDir
		}

		messages := make([]agentTypes.Message, 0, len(req.Messages))
		for _, msg := range req.Messages {
			if msg.Role == "system" {
				continue
			}
			messages = append(messages, msg)
		}
		req.Messages = messages

		input := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role != "user" {
				continue
			}
			if str, ok := req.Messages[i].Content.(string); ok {
				input = str
				break
			}
		}

		if strings.TrimSpace(input) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{
				"message": "no user message",
				"type":    "invalid_request_error",
			}})
			return
		}

		sessionID := sessionID(req.Messages)
		go sessionManager.Clean()

		events := make(chan agentTypes.Event, 64)
		ctx := c.Request.Context()
		wrapped := sessionManager.Wrap(ctx, sessionID, events, 64)

		go func() {
			defer close(wrapped)
			run(ctx, req, sessionID, input, wrapped)
		}()

		id := "chatcmpl-" + utils.UUID()
		created := time.Now().Unix()
		if req.Stream {
			stream(c, id, created, req.Model, events)
		} else {
			collect(c, id, created, req.Model, events)
		}
	}
}

func extractWorkDirFromZed(messages []agentTypes.Message) string {
	for _, msg := range messages {
		if msg.Role != "system" {
			continue
		}

		str, ok := msg.Content.(string)
		if !ok {
			continue
		}
		if matches := zedWorkDirRegex.FindStringSubmatch(str); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func sessionID(messages []agentTypes.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "user" {
			if s, ok := msg.Content.(string); ok {
				sb.WriteString(msg.Role)
				sb.WriteByte(':')
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
		}
		if sb.Len() > 512 {
			break
		}
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return "chatcmpl-" + hex.EncodeToString(sum[:])[:16]
}

func collect(c *gin.Context, id string, created int64, model string, events <-chan agentTypes.Event) {
	var textBuf strings.Builder
	var usage agentTypes.Usage
	var streamErr error
	for ev := range events {
		switch ev.Type {
		case agentTypes.EventText:
			textBuf.WriteString(ev.Text)

		case agentTypes.EventDone:
			if ev.Usage != nil {
				usage = *ev.Usage
			}

		case agentTypes.EventError:
			streamErr = ev.Err
		}
	}
	if streamErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"message": streamErr.Error(), "type": "upstream_error"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []gin.H{{
			"index":         0,
			"message":       gin.H{"role": "assistant", "content": textBuf.String()},
			"finish_reason": "stop",
		}},
		"usage": gin.H{
			"prompt_tokens":     usage.Input,
			"completion_tokens": usage.Output,
			"total_tokens":      usage.Input + usage.Output,
		},
	})
}
