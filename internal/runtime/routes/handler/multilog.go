package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
)

type taggedEvent struct {
	Session string `json:"session"`
	agentTypes.Event
}

func StreamMultiLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.Query("sessions"))
		if raw == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessions query param is required"})
			return
		}

		var sids []string
		for s := range strings.SplitSeq(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				sids = append(sids, s)
			}
		}
		if len(sids) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no valid session ids"})
			return
		}

		h := c.Writer.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		var subs []*pubsub.Subscriber

		for _, sid := range sids {
			connEv := taggedEvent{
				Session: sid,
				Event:   agentTypes.Event{Type: agentTypes.EventConnected, Text: sid},
			}
			if raw, err := json.Marshal(connEv); err == nil {
				fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
			}

			for _, ev := range sessionLog.RecentEvents(sid, 100) {
				te := taggedEvent{Session: sid, Event: ev}
				if raw, err := json.Marshal(te); err == nil {
					fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
				}
			}
		}
		c.Writer.Flush()

		merged := make(chan taggedEvent, 128)
		for _, sid := range sids {
			sub := pubsub.Sub(sid, 64)
			subs = append(subs, sub)

			go func(id string, s *pubsub.Subscriber) {
				for ev := range s.Events() {
					select {
					case merged <- taggedEvent{Session: id, Event: ev}:
					default:
					}
				}
			}(sid, sub)
		}

		defer func() {
			for _, s := range subs {
				s.Close()
			}
		}()

		ctx := c.Request.Context()
		heartbeat := time.NewTicker(logHeartbeat)
		defer heartbeat.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case te, ok := <-merged:
				if !ok {
					return
				}
				raw, err := json.Marshal(te)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", raw); err != nil {
					return
				}
				c.Writer.Flush()
			case <-heartbeat.C:
				if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}
