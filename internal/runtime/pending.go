package runtime

import (
	"context"
	"slices"
	"strings"
	"sync"
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
	Detail      string   `json:"detail,omitempty"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

type UserPayload struct {
	Questions []Question `json:"questions"`
}

type Request struct {
	ID         string
	Kind       Kind
	SessionID  string
	TaskHash   string
	Origin     string
	DeliverTo  string
	ToolName   string
	ToolArgs   string
	Restricted []string
	Explain    string
	AskUser    *UserPayload
	Inline     bool
	Ctx        context.Context
	EnqueueAt  time.Time
}

type Reply struct {
	Approve   bool
	Verified  bool
	Skip      bool
	Remember  bool
	AllowTurn bool
	Reason    string
	Answers   []any
	ExitCode  int
	Error     error
}

type entry struct {
	req       Request
	replyCh   chan Reply
	claimed   bool
	async     bool
	onResolve func(Reply)
}

type listenerEntry struct {
	prefix string
	notify chan struct{}
	match  func(context.Context) bool
}

var (
	mu      sync.Mutex
	entries = map[string]*entry{}

	listenerMu sync.RWMutex
	listeners  []*listenerEntry
)

func RegisterListener(prefix string) (<-chan struct{}, func()) {
	return RegisterListenerMatch(prefix, nil)
}

func RegisterListenerMatch(prefix string, match func(context.Context) bool) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	le := &listenerEntry{prefix: prefix, notify: ch, match: match}

	listenerMu.Lock()
	listeners = append(listeners, le)
	listenerMu.Unlock()

	select {
	case ch <- struct{}{}:
	default:
	}

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			listenerMu.Lock()
			for i, l := range listeners {
				if l == le {
					listeners = slices.Delete(listeners, i, i+1)
					break
				}
			}
			listenerMu.Unlock()
		})
	}
}

var knownOrigins = []string{"cli-", "chat-", "tg-", "dc-"}

func OriginOf(sessionID string) string {
	for _, prefix := range knownOrigins {
		if strings.HasPrefix(sessionID, prefix) {
			return prefix
		}
	}
	return ""
}

func HasListener(ctx context.Context, origin string) bool {
	listenerMu.RLock()
	defer listenerMu.RUnlock()
	for _, l := range listeners {
		if l.prefix != "" && !strings.HasPrefix(origin, l.prefix) {
			continue
		}
		if l.match == nil || l.match(ctx) {
			return true
		}
	}
	return false
}

func Ask(ctx context.Context, req Request) (Reply, error) {
	if req.ID == "" {
		req.ID = go_pkg_utils.UUID()
	}
	if req.Origin == "" {
		req.Origin = OriginOf(req.SessionID)
	}
	if req.DeliverTo == "" {
		req.DeliverTo = req.SessionID
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
	signalFor(req.Origin)

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

func PickNextMatch(prefix string, accept func(Request) bool) (id string, req Request, ok bool) {
	mu.Lock()
	defer mu.Unlock()

	var chosen *entry
	var chosenID string
	for entryID, e := range entries {
		if e.claimed {
			continue
		}
		if prefix != "" && !strings.HasPrefix(e.req.Origin, prefix) {
			continue
		}
		if e.req.Ctx != nil && e.req.Ctx.Err() != nil {
			continue
		}
		if accept != nil && !accept(e.req) {
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

func AskUser(req Request, onResolve func(Reply)) (string, error) {
	if req.ID == "" {
		req.ID = go_pkg_utils.UUID()
	}
	if req.Origin == "" {
		req.Origin = OriginOf(req.SessionID)
	}
	if req.DeliverTo == "" {
		req.DeliverTo = req.SessionID
	}
	req.Ctx = context.Background()
	req.EnqueueAt = time.Now()

	e := &entry{
		req:       req,
		replyCh:   make(chan Reply, 1),
		async:     true,
		onResolve: onResolve,
	}

	mu.Lock()
	entries[req.ID] = e
	mu.Unlock()
	signalFor(req.Origin)

	return req.ID, nil
}

func Resolve(id string, r Reply) {
	mu.Lock()
	e, ok := entries[id]
	if ok && e.async {
		delete(entries, id)
	}
	mu.Unlock()
	if !ok {
		return
	}
	if e.async {
		if e.onResolve != nil {
			go e.onResolve(r)
		}
		return
	}
	select {
	case e.replyCh <- r:
	default:
	}
}

func DropTask(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for id, e := range entries {
		if e.req.TaskHash != taskHash {
			continue
		}
		if sessionID != "" && e.req.SessionID != sessionID && e.req.DeliverTo != sessionID {
			continue
		}
		delete(entries, id)
	}
}

func EntryExists(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := entries[id]
	return ok
}

func signalFor(origin string) {
	listenerMu.RLock()
	defer listenerMu.RUnlock()
	for _, l := range listeners {
		if l.prefix != "" && !strings.HasPrefix(origin, l.prefix) {
			continue
		}
		select {
		case l.notify <- struct{}{}:
		default:
		}
	}
}
