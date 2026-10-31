package historyStore

import (
	"path/filepath"
	"strings"
	"sync"
)

var taskOrigin sync.Map

func originKey(taskID, dir, name string) string {
	return taskID + "\x00" + filepath.Join(dir, name)
}

func rememberOrigin(taskID string, c Change) Change {
	if taskID == "" {
		return c
	}

	key := originKey(taskID, c.dir, c.name)
	if kept, ok := taskOrigin.Load(key); ok {
		return kept.(Change)
	}
	taskOrigin.Store(key, c)
	return c
}

func renameTaskOrigin(oldID, newID string) {
	prefix := oldID + "\x00"
	taskOrigin.Range(func(key, value any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			taskOrigin.Store(newID+"\x00"+strings.TrimPrefix(k, prefix), value)
			taskOrigin.Delete(k)
		}
		return true
	})
}

func ClearTaskOrigin(taskID string) {
	if taskID == "" {
		return
	}

	prefix := taskID + "\x00"
	taskOrigin.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			taskOrigin.Delete(k)
		}
		return true
	})
}
