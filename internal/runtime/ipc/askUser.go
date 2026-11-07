package ipc

import (
	"context"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/sudo"
)

func (c *conn) askUser(ctx context.Context) {
	notify, unregister := runtime.RegisterListenerMatch("cli-", c.owns)
	defer unregister()

	for {
		for {
			id, req, ok := runtime.PickNextMatch("cli-", func(req runtime.Request) bool {
				return c.owns(req.Ctx)
			})
			if !ok {
				break
			}
			ask := &Ask{
				ID:         id,
				Kind:       req.Kind,
				ToolName:   req.ToolName,
				ToolArgs:   req.ToolArgs,
				Restricted: req.Restricted,
				Explain:    req.Explain,
			}
			if len(req.Restricted) > 0 {
				ask.NeedPassword = !sudo.Cached(ctx)
			}
			if req.AskUser != nil {
				ask.Questions = req.AskUser.Questions
			}
			c.askMu.Lock()
			c.asks[id] = req
			c.askMu.Unlock()
			c.write(Frame{Type: FrameAsk, SessionID: req.DeliverTo, Ask: ask})
		}

		select {
		case <-ctx.Done():
			return
		case <-notify:
		}
	}
}
