package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

type PendingAuth[MsgID any] struct {
	Code        string
	PromptMsgID MsgID
}

type PendingRegistry[ChatID comparable, MsgID any] struct {
	mu    sync.Mutex
	items map[ChatID]*PendingAuth[MsgID]
}

func NewPendingRegistry[ChatID comparable, MsgID any]() *PendingRegistry[ChatID, MsgID] {
	return &PendingRegistry[ChatID, MsgID]{items: make(map[ChatID]*PendingAuth[MsgID])}
}

func (r *PendingRegistry[ChatID, MsgID]) Set(id ChatID, code string, promptMsgID MsgID) {
	r.mu.Lock()
	r.items[id] = &PendingAuth[MsgID]{Code: code, PromptMsgID: promptMsgID}
	r.mu.Unlock()
}

func (r *PendingRegistry[ChatID, MsgID]) Get(id ChatID) (*PendingAuth[MsgID], bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.items[id]
	return p, ok
}

func (r *PendingRegistry[ChatID, MsgID]) Clear(id ChatID) {
	r.mu.Lock()
	delete(r.items, id)
	r.mu.Unlock()
}

func GenerateAuthCode() (string, error) {
	num, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("rand.Reader: %w", err)
	}
	return fmt.Sprintf("%06d", num.Int64()), nil
}

func ParseChatID(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	start := 0
	if line[0] == '-' {
		start = 1
	}
	if idx := strings.IndexByte(line[start:], '-'); idx >= 0 {
		return line[:start+idx]
	}
	return line
}

func ParseChatName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	start := 0
	if line[0] == '-' {
		start = 1
	}
	if idx := strings.IndexByte(line[start:], '-'); idx >= 0 {
		return line[start+idx+1:]
	}
	return ""
}

func IsAuthorized(path, target string) bool {
	if !go_pkg_filesystem_reader.Exists(path) {
		return false
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(text, "\n") {
		if ParseChatID(line) == target {
			return true
		}
	}
	return false
}

func LookupChatName(path, target string) string {
	if !go_pkg_filesystem_reader.Exists(path) {
		return ""
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(text, "\n") {
		if ParseChatID(line) == target {
			return ParseChatName(line)
		}
	}
	return ""
}
