package tui

import (
	"context"

	"github.com/pardnchiu/agenvoy/internal/pending"
)

type Pending struct {
	id      string
	request pending.Request
}

func newPendingChannel(ctx context.Context) {
	pending.Active.Store(true)
	defer pending.Active.Store(false)

	for {
		for {
			id, req, ok := pending.PickNext()
			if !ok {
				break
			}
			if req.Ctx != nil {
				if err := req.Ctx.Err(); err != nil {
					pending.Resolve(id, pending.Reply{Error: err})
					continue
				}
			}
			send(Pending{
				id:      id,
				request: req,
			})
		}

		select {
		case <-ctx.Done():
			return
		case <-pending.Notify:
		}
	}
}
