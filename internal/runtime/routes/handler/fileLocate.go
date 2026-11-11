package handler

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const locateBudget = 8 * time.Second

func LocateFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		if name == "" || name != filepath.Base(name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name must be a bare file name"})
			return
		}
		dir := c.Query("dir") == "1" || c.Query("dir") == "true"
		children := c.QueryArray("child")

		var size int64
		if !dir {
			parsed, err := strconv.ParseInt(c.Query("size"), 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "size must be a number"})
				return
			}
			size = parsed
		}
		mtime, _ := strconv.ParseInt(c.Query("mtime"), 10, 64)

		home, err := os.UserHomeDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		list := []string{"Desktop", "Downloads", "Documents", "Pictures", "Movies", "Music", "Library/Mobile Documents"}
		skip := map[string]bool{filepath.Join(home, "Library"): true}
		roots := make([]string, 0, len(list)+1)
		for _, one := range list {
			root := filepath.Join(home, one)
			roots = append(roots, root)
			skip[root] = true
		}
		roots = append(roots, home)

		deadline := time.Now().Add(locateBudget)
		for _, root := range roots {
			if paths := locateIn(root, skip, name, size, mtime, dir, children, deadline); len(paths) > 0 {
				c.JSON(http.StatusOK, gin.H{"paths": paths})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"paths": []string{}})
	}
}

func locateIn(root string, skip map[string]bool, name string, size, mtime int64, dir bool, children []string, deadline time.Time) []string {
	var paths []string
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if time.Now().After(deadline) {
			return filepath.SkipAll
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || skip[path]) {
				return filepath.SkipDir
			}
			if !dir || path == root || entry.Name() != name {
				return nil
			}
			if !holdsChildren(path, children) {
				return nil
			}
			if !filesystem.IsSensitivePath(path) {
				paths = append(paths, path)
			}
			return filepath.SkipDir
		}
		if dir || entry.Name() != name || !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() != size || !sameMoment(info.ModTime(), mtime) {
			return nil
		}
		if !filesystem.IsSensitivePath(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

func holdsChildren(dir string, children []string) bool {
	for _, one := range children {
		if one == "" || one != filepath.Base(one) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, one)); err != nil {
			return false
		}
	}
	return true
}

func sameMoment(at time.Time, ms int64) bool {
	if ms <= 0 {
		return true
	}
	diff := at.UnixMilli() - ms
	return diff >= -1000 && diff <= 1000
}
