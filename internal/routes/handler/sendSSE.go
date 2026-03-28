package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func sendSSE(c *gin.Context, sessionID string, input string, events <-chan agentTypes.Event) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return
	}

	sessionEvent, err := json.Marshal(map[string]string{
		"event":      "cehck session",
		"session_id": sessionID,
		"input":      input,
	})
	if err != nil {
		return
	}
	utils.EventLog("[HTTP]", agentTypes.Event{
		Type: agentTypes.EventText,
		Text: input,
	}, sessionID, input)
	fmt.Fprintf(writer, "data: %s\n\n", sessionEvent)
	flusher.Flush()

	for event := range events {
		if event.Type == agentTypes.EventToolConfirm && event.ReplyCh != nil {
			event.ReplyCh <- true
			continue
		}

		data, err := json.Marshal(event)
		if err != nil {
			continue
		}

		utils.EventLog("[HTTP]", event, sessionID, "")

		fmt.Fprintf(writer, "data: %s\n\n", data)
		flusher.Flush()

		if event.Type == agentTypes.EventDone || event.Type == agentTypes.EventError {
			break
		}
	}
}
