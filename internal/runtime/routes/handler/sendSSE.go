package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func sendSSE(c *gin.Context, sessionID string, input string, events <-chan agenttypes.Event) {
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
	utils.EventLog("[HTTP]", agenttypes.Event{
		Type: agenttypes.EventText,
		Text: input,
	}, sessionID, input)
	fmt.Fprintf(writer, "data: %s\n\n", sessionEvent)
	flusher.Flush()

	for event := range events {
		if event.Type == agenttypes.EventText {
			continue
		}

		if event.Err != nil && event.Text == "" {
			event.Text = event.Err.Error()
		}
		raw, err := json.Marshal(event)
		if err != nil {
			continue
		}

		utils.EventLog("[HTTP]", event, sessionID, "")

		fmt.Fprintf(writer, "data: %s\n\n", raw)
		flusher.Flush()

		if event.Type == agenttypes.EventDone || event.Type == agenttypes.EventError || event.Type == agenttypes.EventCanceled {
			break
		}
	}
}
