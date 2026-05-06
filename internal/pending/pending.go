package pending

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type Kind string

const (
	KindToolConfirm Kind = "tool_confirm"
	KindAskUser     Kind = "ask_user"
)

type Question struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

type UserPayload struct {
	Questions []Question `json:"questions"`
}

type Request struct {
	ID        string
	Kind      Kind
	SessionID string
	ToolName  string
	ToolArgs  string
	AskUser   *UserPayload
	Ctx       context.Context
	EnqueueAt time.Time
}

type Reply struct {
	Approve bool
	Skip    bool
	Answers []any
	Err     error
}

type entry struct {
	req     Request
	replyCh chan Reply
	claimed bool
}

var (
	mu      sync.Mutex
	entries = map[string]*entry{}
	notify  = make(chan struct{}, 1)

	Active atomic.Bool
	Notify <-chan struct{} = notify
)

func Ask(ctx context.Context, req Request) (Reply, error) {
	if req.ID == "" {
		req.ID = go_pkg_utils.UUID()
	}
	req.Ctx = ctx
	req.EnqueueAt = time.Now()

	e := &entry{
		req:     req,
		replyCh: make(chan Reply, 1),
	}

	mu.Lock()
	entries[req.ID] = e
	mu.Unlock()
	signal()

	defer func() {
		mu.Lock()
		delete(entries, req.ID)
		mu.Unlock()
	}()

	select {
	case r := <-e.replyCh:
		return r, nil
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	}
}

func PickNext() (id string, req Request, ok bool) {
	mu.Lock()
	defer mu.Unlock()

	var chosen *entry
	var chosenID string
	for entryID, e := range entries {
		if e.claimed {
			continue
		}
		if e.req.Ctx != nil && e.req.Ctx.Err() != nil {
			continue
		}
		if chosen == nil || e.req.EnqueueAt.Before(chosen.req.EnqueueAt) {
			chosen = e
			chosenID = entryID
		}
	}
	if chosen == nil {
		return "", Request{}, false
	}
	chosen.claimed = true
	return chosenID, chosen.req, true
}

func Resolve(id string, r Reply) {
	mu.Lock()
	e, ok := entries[id]
	mu.Unlock()
	if !ok {
		return
	}
	select {
	case e.replyCh <- r:
	default:
	}
}

func Snapshot() []Request {
	mu.Lock()
	out := make([]Request, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.req)
	}
	mu.Unlock()
	slices.SortFunc(out, func(a, b Request) int {
		switch {
		case a.EnqueueAt.Before(b.EnqueueAt):
			return -1
		case a.EnqueueAt.After(b.EnqueueAt):
			return 1
		default:
			return 0
		}
	})
	return out
}

func signal() {
	select {
	case notify <- struct{}{}:
	default:
	}
}
