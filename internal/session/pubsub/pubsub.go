package pubsub

import (
	"context"
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

var (
	mu   sync.RWMutex
	subs = map[string][]*Subscriber{}
)

func Sub(sessionID string, length int) *Subscriber {
	if length <= 0 {
		length = 32
	}

	sub := &Subscriber{
		channel:   make(chan agentTypes.Event, length),
		sessionID: sessionID,
	}

	mu.Lock()
	subs[sessionID] = append(subs[sessionID], sub)
	mu.Unlock()
	return sub
}

func Pub(sessionID string, event agentTypes.Event) {
	if sessionID == "" {
		return
	}

	mu.RLock()
	list := subs[sessionID]
	mu.RUnlock()

	for _, s := range list {
		s.send(event)
	}
}

func Wrap(ctx context.Context, sessionID string, channel chan agentTypes.Event, length int) chan agentTypes.Event {
	if sessionID == "" {
		return channel
	}

	if length <= 0 {
		length = cap(channel)
		if length <= 0 {
			length = 32
		}
	}

	src := make(chan agentTypes.Event, length)
	go func() {
		defer close(channel)
		for event := range src {
			Pub(sessionID, event)

			select {
			case channel <- event:
			case <-ctx.Done():
				for next := range src {
					Pub(sessionID, next)
				}
				return
			}
		}
	}()
	return src
}
