package record

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

const (
	downloadMaxAge = 30 * 24 * time.Hour
)

func CleanDownload() {
	src := filepath.Join(filesystem.AgenvoyDir, "download")
	if !go_pkg_filesystem_reader.IsDir(src) {
		return
	}

	cutoff := time.Now().Add(-downloadMaxAge)
	entries, err := os.ReadDir(src)
	if err != nil {
		slog.Warn("record CleanDownload ReadDir",
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		if entry.Name() == ".Trash" {
			continue
		}
		path := filepath.Join(src, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		dst := filepath.Join(filesystem.DownloadTrashDir, entry.Name())
		if go_pkg_filesystem_reader.Exists(dst) {
			dst = filepath.Join(filesystem.DownloadTrashDir, fmt.Sprintf("%s-%d%s",
				nameWithoutExt(entry.Name()),
				time.Now().Unix(),
				filepath.Ext(entry.Name())))
		}
		if err := os.Rename(path, dst); err != nil {
			slog.Warn("record CleanDownload Rename",
				slog.String("src", path),
				slog.String("error", err.Error()))
		}
	}
}

func nameWithoutExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}
