package tui

import (
	"context"

	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type Pending struct {
	id      string
	request runtime.Request
}

func newPendingChannel(ctx context.Context) {
	unregister := runtime.RegisterListener("")
	defer unregister()

	for {
		for {
			id, next, ok := runtime.PickNext("")
			if !ok {
				break
			}
			if next.Ctx != nil {
				if err := next.Ctx.Err(); err != nil {
					runtime.Resolve(id, runtime.Reply{Error: err})
					continue
				}
			}
			send(Pending{
				id:      id,
				request: next,
			})
		}

		select {
		case <-ctx.Done():
			return
		case <-runtime.Notify:
		}
	}
}
