package denied

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type pathSet struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

var cache sync.Map

func get(sessionID string) *pathSet {
	if v, ok := cache.Load(sessionID); ok {
		return v.(*pathSet)
	}
	ps := &pathSet{paths: map[string]struct{}{}}
	actual, _ := cache.LoadOrStore(sessionID, ps)
	return actual.(*pathSet)
}

func Register(sessionID, absPath string) {
	absPath = filepath.Clean(absPath)
	if absPath == "" || absPath == "/" {
		return
	}
	ps := get(sessionID)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.paths[absPath] = struct{}{}
}

func Hit(sessionID, absPath string) (string, bool) {
	absPath = filepath.Clean(absPath)
	ps := get(sessionID)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	sep := string(filepath.Separator)
	for p := range ps.paths {
		if absPath == p || strings.HasPrefix(absPath, p+sep) {
			return p, true
		}
	}
	return "", false
}

func IsPermission(err error) bool {
	if err == nil {
		return false
	}
	return os.IsPermission(err) || errors.Is(err, fs.ErrPermission)
}
