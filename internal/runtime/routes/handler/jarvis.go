package handler

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

const jarvisSessionID = "jarvis"

//go:embed static/index.html
var jarvisShellHTML string

//go:embed static/index.css
var jarvisCSS string

//go:embed static/index.js
var jarvisJS string

const defaultPageHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>agenvoy</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:#06090d;font-family:Inter,-apple-system,sans-serif}
.greeting{text-align:center;color:#e2e8f0}
.greeting h1{font-size:28px;font-weight:600;margin-bottom:8px}
.greeting p{font-size:15px;color:#64748b}
</style>
</head>
<body>
<div class="greeting">
<h1>Agenvoy</h1>
<p>Your personal agent — type below to start.</p>
</div>
</body>
</html>`

var jarvisStaticDir string

func InitJarvisStatic() {
	pageDir := filepath.Join(filesystem.SessionDir(jarvisSessionID), "page")
	_ = go_pkg_filesystem.CheckDir(pageDir, true)

	link := filepath.Join(pageDir, "static")
	target := filepath.Join(filesystem.AgenvoyDir, "download")

	if go_pkg_filesystem_reader.Exists(link) {
		resolved, err := os.Readlink(link)
		if err == nil && resolved == target {
			jarvisStaticDir = link
		} else {
			_ = os.Remove(link)
			if err := os.Symlink(target, link); err != nil {
				slog.Warn("jarvis symlink static",
					slog.String("link", link),
					slog.String("target", target),
					slog.String("error", err.Error()))
			} else {
				jarvisStaticDir = link
			}
		}
	} else {
		if err := os.Symlink(target, link); err != nil {
			slog.Warn("jarvis symlink static",
				slog.String("link", link),
				slog.String("target", target),
				slog.String("error", err.Error()))
		} else {
			jarvisStaticDir = link
		}
	}

	srcDir := filepath.Join(pageDir, "src")
	_ = go_pkg_filesystem.CheckDir(srcDir, true)

	_ = go_pkg_filesystem.WriteFile(filepath.Join(srcDir, "index.css"), jarvisCSS, 0644)
	_ = go_pkg_filesystem.WriteFile(filepath.Join(srcDir, "index.js"), jarvisJS, 0644)
}

func JarvisStatic() gin.HandlerFunc {
	dir := filepath.Join(filesystem.AgenvoyDir, "download")
	return func(c *gin.Context) {
		fp := c.Param("filepath")
		if fp == "" || fp == "/" {
			c.Status(http.StatusNotFound)
			return
		}

		absPath := filepath.Join(dir, filepath.Clean(fp))
		if !go_pkg_filesystem_reader.Exists(absPath) {
			c.Status(http.StatusNotFound)
			return
		}

		c.File(absPath)
	}
}

func JarvisSrc() gin.HandlerFunc {
	srcDir := filepath.Join(filesystem.SessionDir(jarvisSessionID), "page", "src")
	return func(c *gin.Context) {
		fp := c.Param("filepath")
		if fp == "" || fp == "/" {
			c.Status(http.StatusNotFound)
			return
		}

		absPath := filepath.Join(srcDir, filepath.Clean(fp))
		if !go_pkg_filesystem_reader.Exists(absPath) {
			c.Status(http.StatusNotFound)
			return
		}

		c.File(absPath)
	}
}

func Jarvis() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(jarvisShellHTML))
	}
}

func JarvisReset() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func JarvisListener() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		sub := pubsub.Sub(jarvisSessionID, 16)
		defer sub.Close()

		ctx := c.Request.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.Events():
				if !ok {
					return
				}
				if ev.Type == agentTypes.EventTextDone || ev.Type == agentTypes.EventDone {
					ts := latestPageTS()
					fmt.Fprintf(c.Writer, "data: {\"type\":%q,\"ts\":%q}\n\n", ev.Type, ts)
					c.Writer.Flush()
				}
			}
		}
	}
}

func JarvisPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		ts := strings.TrimSpace(c.Query("ts"))
		if ts == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(defaultPageHTML))
			return
		}

		db := torii.DB(torii.DBJarvisPage)
		entry, ok := db.Get(jarvisSessionID + ":" + ts)
		if !ok {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(defaultPageHTML))
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(entry.Value()))
	}
}

func JarvisHistory() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := torii.DB(torii.DBJarvisPage)
		entry, ok := db.Get(jarvisSessionID + ":history")
		if !ok {
			c.JSON(http.StatusOK, gin.H{"history": []string{}})
			return
		}

		var list []string
		if err := json.Unmarshal([]byte(entry.Value()), &list); err != nil {
			c.JSON(http.StatusOK, gin.H{"history": []string{}})
			return
		}

		// filter: only return ts entries that still exist in DB
		var valid []string
		for _, ts := range list {
			if _, exists := db.Get(jarvisSessionID + ":" + ts); exists {
				valid = append(valid, ts)
			}
		}

		if valid == nil {
			valid = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"history": valid})
	}
}

func latestPageTS() string {
	db := torii.DB(torii.DBJarvisPage)
	entry, ok := db.Get(jarvisSessionID + ":history")
	if !ok {
		return ""
	}

	var list []string
	if err := json.Unmarshal([]byte(entry.Value()), &list); err != nil || len(list) == 0 {
		return ""
	}

	return list[len(list)-1]
}
