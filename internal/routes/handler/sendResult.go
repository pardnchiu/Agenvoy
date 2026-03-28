package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type Response struct {
	Text      string            `json:"text"`
	Model     string            `json:"model,omitempty"`
	Usage     *agentTypes.Usage `json:"usage,omitempty"`
	SessionID string            `json:"session_id"`
}

func sendResult(c *gin.Context, sessionID string, input string, events <-chan agentTypes.Event) {
	var sb strings.Builder
	var resp Response
	resp.SessionID = sessionID

	utils.EventLog("[HTTP]", agentTypes.Event{
		Type: agentTypes.EventText,
		Text: input,
	}, sessionID, input)

	ctx := c.Request.Context()
	for event := range events {
		if ctx.Err() != nil {
			c.JSON(http.StatusRequestTimeout, gin.H{"error": ctx.Err().Error()})
			return
		}

		utils.EventLog("[HTTP]", event, sessionID, "")

		switch event.Type {
		case agentTypes.EventToolConfirm:
			if event.ReplyCh != nil {
				event.ReplyCh <- true
			}
		case agentTypes.EventText:
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(event.Text)
		case agentTypes.EventDone:
			resp.Model = event.Model
			resp.Usage = event.Usage
		case agentTypes.EventError:
			c.JSON(http.StatusInternalServerError, gin.H{"error": event.Err.Error()})
			return
		}
	}

	resp.Text = sb.String()
	c.JSON(http.StatusOK, resp)
}
