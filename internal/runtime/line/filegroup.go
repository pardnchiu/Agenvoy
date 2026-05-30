package line

import (
	"context"
	"log/slog"
	"sync"
	"time"

	go_bot_line "github.com/pardnchiu/go-bot/line"
)

const (
	fileGroupWindow  = 3 * time.Second
	fileGroupMaxWait = 30 * time.Second
)

type fileGroup struct {
	inputs    []go_bot_line.Input
	timer     *time.Timer
	firstSeen time.Time
}

type fileGroupBuffer struct {
	mu     sync.Mutex
	groups map[string]*fileGroup
}

func newFileGroupBuffer() *fileGroupBuffer {
	return &fileGroupBuffer{groups: make(map[string]*fileGroup)}
}

func (fb *fileGroupBuffer) offer(b *Bot, key string, in go_bot_line.Input, isAttachment bool) bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	group, ok := fb.groups[key]
	if !isAttachment && !ok {
		return false
	}
	if !ok {
		group = &fileGroup{firstSeen: time.Now()}
		fb.groups[key] = group
	}
	group.inputs = append(group.inputs, in)

	if group.timer != nil {
		group.timer.Stop()
	}
	delay := min(fileGroupWindow, max(fileGroupMaxWait-time.Since(group.firstSeen), 0))
	group.timer = time.AfterFunc(delay, func() { fb.flush(b, key) })
	return true
}

func (fb *fileGroupBuffer) flush(b *Bot, key string) {
	fb.mu.Lock()
	group, ok := fb.groups[key]
	if ok {
		delete(fb.groups, key)
	}
	fb.mu.Unlock()
	if !ok || len(group.inputs) == 0 {
		return
	}

	primary := group.inputs[0]
	if err := run(context.Background(), b, primary, group.inputs); err != nil {
		slog.Warn("run",
			slog.String("source", sourceID(primary)),
			slog.String("error", err.Error()))
	}
}
