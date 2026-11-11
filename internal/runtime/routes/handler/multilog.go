package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	configstatus "github.com/pardnchiu/agenvoy/internal/session/config/status"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	internalutils "github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	mergeBlockWait = 250 * time.Millisecond
	logHeartbeat   = 25 * time.Second
)

type taggedEvent struct {
	Session string `json:"session"`
	agenttypes.Event
	Display         string   `json:"tool_display,omitempty"`
	ConfirmTitle    string   `json:"confirm_title,omitempty"`
	RestrictedLines []string `json:"restricted_lines,omitempty"`
	RestrictedNote  string   `json:"restricted_note,omitempty"`
	Trace           string   `json:"trace,omitempty"`
}

type wireEvent struct {
	agenttypes.Event
	Display         string   `json:"tool_display,omitempty"`
	ConfirmTitle    string   `json:"confirm_title,omitempty"`
	RestrictedLines []string `json:"restricted_lines,omitempty"`
	RestrictedNote  string   `json:"restricted_note,omitempty"`
	Trace           string   `json:"trace,omitempty"`
}

func skipEvent(ev agenttypes.Event) bool {
	switch ev.Type {
	case agenttypes.EventToolProgress:
		return true

	case agenttypes.EventToolCall, agenttypes.EventToolResult, agenttypes.EventToolSkipped,
		agenttypes.EventToolCallStart, agenttypes.EventToolCallText, agenttypes.EventToolCallEnd,
		agenttypes.EventToolConfirm:
		return internalutils.HideToolEvent(ev.ToolName, ev.ToolArgs)
	}
	return false
}

func toWire(ev agenttypes.Event) wireEvent {
	wire := wireEvent{Event: ev, Trace: internalutils.FormatTraceLine(ev)}
	switch ev.Type {
	case agenttypes.EventToolCall:
		wire.Display = internalutils.FormatToolEvent(ev.ToolName, ev.ToolArgs)
	case agenttypes.EventToolConfirm:
		msg := internalutils.FormatConfirm(ev.ToolName, ev.ToolArgs, ev.Restricted, ev.PasswordCached)
		wire.Display = msg.Display
		wire.ConfirmTitle = msg.Title
		wire.RestrictedLines = msg.RestrictedLines
		wire.RestrictedNote = msg.RestrictedNote
	}
	return wire
}

func toTagged(sessionID string, ev agenttypes.Event) taggedEvent {
	wire := toWire(ev)
	return taggedEvent{
		Session:         sessionID,
		Event:           ev,
		Display:         wire.Display,
		ConfirmTitle:    wire.ConfirmTitle,
		RestrictedLines: wire.RestrictedLines,
		RestrictedNote:  wire.RestrictedNote,
		Trace:           wire.Trace,
	}
}

type connectedFrame struct {
	Session string `json:"session,omitempty"`
	agenttypes.Event
	State string `json:"state,omitempty"`
}

func newConnectedFrame(sessionID string) connectedFrame {
	status := configstatus.Get(sessionID)
	return connectedFrame{
		Session: sessionID,
		Event:   agenttypes.Event{Type: agenttypes.EventConnected, Text: sessionID},
		State:   status.State,
	}
}

func StreamMultiLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		var sids []string
		for s := range strings.SplitSeq(strings.TrimSpace(c.Query("sessions")), ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				sids = append(sids, s)
			}
		}

		withDaemon := internalutils.IsLoopback(c.Request.RemoteAddr) && c.DefaultQuery("daemon", "1") != "0"
		if len(sids) == 0 && !withDaemon {
			c.JSON(http.StatusBadRequest, gin.H{"error": "sessions is required when daemon=0 or the caller is remote"})
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

		replay := c.DefaultQuery("replay", "1") != "0"

		for _, sid := range sids {
			if raw, err := json.Marshal(newConnectedFrame(sid)); err == nil {
				fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
			}

			if !replay {
				continue
			}

			for _, ev := range sessionlog.RecentEvents(sid, 512) {
				if skipEvent(ev) {
					continue
				}
				te := toTagged(sid, ev)
				if raw, err := json.Marshal(te); err == nil {
					fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
				}
			}
		}
		c.Writer.Flush()

		for _, sid := range sids {
			for _, ev := range OutstandingConfirms(sid) {
				if raw, err := json.Marshal(toTagged(sid, ev)); err == nil {
					fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
				}
			}
		}
		c.Writer.Flush()

		ctx := c.Request.Context()
		merged := make(chan taggedEvent, 1024)
		var fanInDropped atomic.Int64

		if withDaemon {
			sids = append(sids, configs.DAEMON_LOG_CHANNEL)
		}

		for _, sid := range sids {
			sub := pubsub.Sub(sid, 1024)
			subs = append(subs, sub)

			go func(id string, s *pubsub.Subscriber) {
				for ev := range s.Events() {
					if skipEvent(ev) {
						continue
					}
					te := toTagged(id, ev)
					select {
					case merged <- te:
						continue
					default:
					}

					timer := time.NewTimer(mergeBlockWait)
					select {
					case merged <- te:
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
						n := fanInDropped.Add(1)
						slog.Debug("multilog fan-in overflow, event dropped",
							slog.String("session", id),
							slog.String("event", ev.Type.String()),
							slog.Int64("dropped_total", n))
					}
					timer.Stop()
				}
			}(sid, sub)
		}

		defer func() {
			for _, s := range subs {
				s.Close()
			}
		}()

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
				if te.Type == agenttypes.EventDone {
					dropped := fanInDropped.Swap(0)
					for _, s := range subs {
						dropped += s.TakeDropped()
					}
					if dropped > 0 {
						if _, err := fmt.Fprintf(c.Writer, "data: {\"session\":%q,\"type\":\"EventTruncated\",\"dropped\":%d}\n\n", te.Session, dropped); err != nil {
							return
						}
						c.Writer.Flush()
					}
				}
			case <-heartbeat.C:
				if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			}
		}
	}
}
