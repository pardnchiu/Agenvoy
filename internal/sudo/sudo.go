package sudo

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	ttlDuration = time.Hour
)

type NeverOpenConfig struct {
	SystemDirs []string `json:"system_dirs"`
}

var (
	Floor     NeverOpenConfig
	active    atomic.Bool
	expiresAt atomic.Int64
)

func LoadFloor() error {
	if err := json.Unmarshal(configs.NeverOpen, &Floor); err != nil {
		return fmt.Errorf("json.Unmarshal never_open: %w", err)
	}
	return nil
}

func Activate() {
	active.Store(true)
	expiresAt.Store(time.Now().Add(ttlDuration).Unix())
	slog.Warn("sudo mode activated")
}

func Deactivate() {
	active.Store(false)
	expiresAt.Store(0)
	slog.Warn("sudo mode deactivated")
}

func IsActive() bool {
	if !active.Load() {
		return false
	}
	if time.Now().Unix() >= expiresAt.Load() {
		active.Store(false)
		expiresAt.Store(0)
		slog.Warn("sudo mode expired")
		return false
	}
	return true
}

func Refresh() {
	if !IsActive() {
		return
	}
	expiresAt.Store(time.Now().Add(ttlDuration).Unix())
}

func RemainingSeconds() int64 {
	if !active.Load() {
		return 0
	}
	remain := expiresAt.Load() - time.Now().Unix()
	if remain < 0 {
		return 0
	}
	return remain
}

func HitFloor(joined string) (string, bool) {
	for _, dir := range Floor.SystemDirs {
		if strings.HasPrefix(joined, dir+"/") || joined == dir ||
			strings.Contains(joined, " "+dir+"/") || strings.Contains(joined, " "+dir+" ") {
			return dir, true
		}
	}
	return "", false
}
