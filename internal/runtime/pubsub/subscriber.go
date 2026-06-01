package pubsub

import (
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type Subscriber struct {
	channel   chan agentTypes.Event
	sessionID string
	closed    bool
	mu        sync.Mutex
}

func (s *Subscriber) Events() <-chan agentTypes.Event {
	return s.channel
}

func (s *Subscriber) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	s.closed = true
	close(s.channel)
	s.mu.Unlock()

	s.unsub()
}

func (s *Subscriber) send(event agentTypes.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	select {
	case s.channel <- event:
	default:
	}
}

func (s *Subscriber) unsub() {
	mu.Lock()
	defer mu.Unlock()

	list := subs[s.sessionID]
	for i, sub := range list {
		if sub == s {
			subs[s.sessionID] = append(list[:i], list[i+1:]...)
			if len(subs[s.sessionID]) == 0 {
				delete(subs, s.sessionID)
			}
			return
		}
	}
}
