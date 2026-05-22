package session

import (
	"context"
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type Subscriber struct {
	ch        chan agentTypes.Event
	sessionID string
	closed    bool
	mu        sync.Mutex
}

func (s *Subscriber) Events() <-chan agentTypes.Event {
	return s.ch
}

func (s *Subscriber) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.ch)
	s.mu.Unlock()
	unsubscribeRegistry(s)
}

func (s *Subscriber) trySend(ev agentTypes.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- ev:
	default:
		// directly drop if slow
	}
}

var (
	mu   sync.RWMutex
	subs = map[string][]*Subscriber{}
)

func Subscribe(sessionID string, buf int) *Subscriber {
	if buf <= 0 {
		buf = 32
	}
	s := &Subscriber{
		ch:        make(chan agentTypes.Event, buf),
		sessionID: sessionID,
	}
	mu.Lock()
	subs[sessionID] = append(subs[sessionID], s)
	mu.Unlock()
	return s
}

func Publish(sessionID string, ev agentTypes.Event) {
	if sessionID == "" {
		return
	}
	mu.RLock()
	list := subs[sessionID]
	mu.RUnlock()
	for _, s := range list {
		s.trySend(ev)
	}
}

func Wrap(ctx context.Context, sessionID string, dst chan agentTypes.Event, buf int) chan agentTypes.Event {
	if sessionID == "" {
		return dst
	}
	if buf <= 0 {
		buf = cap(dst)
		if buf <= 0 {
			buf = 32
		}
	}
	src := make(chan agentTypes.Event, buf)
	go func() {
		defer close(dst)
		for ev := range src {
			Publish(sessionID, ev)
			select {
			case dst <- ev:
			case <-ctx.Done():
				for next := range src {
					Publish(sessionID, next)
				}
				return
			}
		}
	}()
	return src
}

func unsubscribeRegistry(target *Subscriber) {
	mu.Lock()
	defer mu.Unlock()
	list := subs[target.sessionID]
	for i, s := range list {
		if s == target {
			subs[target.sessionID] = append(list[:i], list[i+1:]...)
			if len(subs[target.sessionID]) == 0 {
				delete(subs, target.sessionID)
			}
			return
		}
	}
}
