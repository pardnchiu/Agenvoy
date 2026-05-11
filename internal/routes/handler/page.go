package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	pageDebounce         = 200 * time.Millisecond
	pagePollInterval     = time.Second
	pageHeartbeatTicks   = 15
	pageWatcherErrorMaxN = 10
)

func PageListener() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
		if !go_pkg_filesystem_reader.Exists(sessionDir) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		pageDir := filesystem.PagePath(sessionID)
		if err := go_pkg_filesystem.CheckDir(pageDir, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ensure page dir: " + err.Error()})
			return
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fsnotify: " + err.Error()})
			return
		}
		defer watcher.Close()

		if err := addPageWatch(watcher, pageDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "watch: " + err.Error()})
			return
		}

		header := c.Writer.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()

		ctx := c.Request.Context()
		poll := time.NewTicker(pagePollInterval)
		defer poll.Stop()

		var (
			debounce   *time.Timer
			debounceCh = make(chan struct{}, 1)
			quietTicks int
			errCount   int
		)
		schedule := func() {
			if debounce == nil {
				debounce = time.AfterFunc(pageDebounce, func() {
					select {
					case debounceCh <- struct{}{}:
					default:
					}
				})
				return
			}
			debounce.Reset(pageDebounce)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if ev.Has(fsnotify.Create) {
					if go_pkg_filesystem_reader.IsDir(ev.Name) {
						if err := watcher.Add(ev.Name); err != nil {
							slog.Warn("page watcher add subdir",
								slog.String("path", ev.Name),
								slog.String("error", err.Error()))
						}
					}
				}
				schedule()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Warn("page watcher error",
					slog.String("session", sessionID),
					slog.String("error", err.Error()))
				errCount++
				if errCount >= pageWatcherErrorMaxN {
					return
				}
			case <-debounceCh:
				quietTicks = 0
				if _, err := fmt.Fprint(c.Writer, "data: 1\n\n"); err != nil {
					return
				}
				c.Writer.Flush()
			case <-poll.C:
				quietTicks++
				if quietTicks >= pageHeartbeatTicks {
					quietTicks = 0
					if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
						return
					}
					c.Writer.Flush()
				}
			}
		}
	}
}

func addPageWatch(watcher *fsnotify.Watcher, root string) error {
	if err := watcher.Add(root); err != nil {
		return err
	}

	subs, err := go_pkg_filesystem_reader.ListDirs(root)
	if err != nil {
		return nil
	}
	for _, d := range subs {
		full := filepath.Join(root, d.Name)
		if err := addPageWatch(watcher, full); err != nil {
			slog.Warn("addPageWatch",
				slog.String("error", err.Error()))
		}
	}
	return nil
}

func PageView() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
		if !go_pkg_filesystem_reader.Exists(sessionDir) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}

		pageDir := filesystem.PagePath(sessionID)
		if !go_pkg_filesystem_reader.IsDir(pageDir) {
			c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
			return
		}

		rel := strings.TrimPrefix(c.Param("filepath"), "/")
		if rel == "" {
			rel = "index.html"
		}

		abs := filepath.Clean(filepath.Join(pageDir, rel))
		rootAbs, err := filepath.Abs(pageDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "abs page dir"})
			return
		}
		absResolved, err := filepath.Abs(abs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
			return
		}
		if absResolved != rootAbs && !strings.HasPrefix(absResolved, rootAbs+string(filepath.Separator)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "path escapes page root"})
			return
		}

		if !go_pkg_filesystem_reader.Exists(absResolved) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		if go_pkg_filesystem_reader.IsDir(absResolved) {
			absResolved = filepath.Join(absResolved, "index.html")
			if !go_pkg_filesystem_reader.Exists(absResolved) {
				c.JSON(http.StatusNotFound, gin.H{"error": "index.html not found"})
				return
			}
		}

		ext := strings.ToLower(filepath.Ext(absResolved))
		if ext == ".html" || ext == ".htm" {
			text, err := go_pkg_filesystem.ReadText(absResolved)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "read html: " + err.Error()})
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(concatReloadScript(text)))
			return
		}

		c.File(absResolved)
	}
}

const reloadScript = `<script>
  (function () {
    var sid = location.pathname.split("/")[1];
    if (!sid) return;
    var es = new EventSource(location.origin + "/v1/page/listener/" + sid);
    es.onmessage = function () { location.reload(); };
  })();
</script>
`

func concatReloadScript(html string) string {
	lower := strings.ToLower(html)
	idx := strings.LastIndex(lower, "</body>")
	if idx == -1 {
		return html + "\n" + reloadScript
	}
	return html[:idx] + reloadScript + html[idx:]
}
