package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var daemonPublishClient = &http.Client{Timeout: 2 * time.Second}

var daemonBaseURL = sync.OnceValue(func() string {
	return "http://127.0.0.1:" + filesystem.Port
})

func publishEventToDaemon(ctx context.Context, sessionID string, ev agentTypes.Event) {
	if sessionID == "" {
		return
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	url := daemonBaseURL() + "/v1/session/" + sessionID + "/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := daemonPublishClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func wrapEventsPublish(ctx context.Context, sessionID string, dst chan agentTypes.Event) chan agentTypes.Event {
	if sessionID == "" {
		return dst
	}
	src := make(chan agentTypes.Event, cap(dst))
	go func() {
		defer close(dst)
		for ev := range src {
			publishEventToDaemon(ctx, sessionID, ev)
			dst <- ev
		}
	}()
	return src
}
