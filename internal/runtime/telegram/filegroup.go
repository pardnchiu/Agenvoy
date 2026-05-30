package telegram

import (
	"context"
	"log/slog"
	"sync"
	"time"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
)

const (
	fileGroupWindow = 1500 * time.Millisecond
)

type fileGroup struct {
	inputs []go_bot_telegram.Input
	timer  *time.Timer
}

type fileGroupBuffer struct {
	mu     sync.Mutex
	groups map[string]*fileGroup
}

func newFileGroupBuffer() *fileGroupBuffer {
	return &fileGroupBuffer{groups: make(map[string]*fileGroup)}
}

func fileGroupID(input go_bot_telegram.Input) string {
	if input.Raw == nil || input.Raw.Message == nil {
		return ""
	}
	return input.Raw.Message.MediaGroupID
}

func (fb *fileGroupBuffer) add(bot *Bot, groupID string, input go_bot_telegram.Input) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	group, ok := fb.groups[groupID]
	if !ok {
		group = &fileGroup{}
		fb.groups[groupID] = group
	}
	group.inputs = append(group.inputs, input)
	if group.timer != nil {
		group.timer.Stop()
	}
	group.timer = time.AfterFunc(fileGroupWindow, func() { fb.flush(bot, groupID) })
}

func (fb *fileGroupBuffer) flush(bot *Bot, groupID string) {
	fb.mu.Lock()
	group, ok := fb.groups[groupID]
	if ok {
		delete(fb.groups, groupID)
	}
	fb.mu.Unlock()
	if !ok || len(group.inputs) == 0 {
		return
	}

	response := group.inputs[0]
	if response.Caption == "" {
		for _, in := range group.inputs {
			if in.Caption != "" {
				response.Caption = in.Caption
				break
			}
		}
	}

	if err := run(context.Background(), bot, response, group.inputs); err != nil {
		slog.Warn("run",
			slog.String("chat", chatName(response)),
			slog.String("error", err.Error()))
	}
}
