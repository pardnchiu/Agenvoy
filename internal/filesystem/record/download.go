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
	downloadMaxAge = 7 * 24 * time.Hour
	trashMaxAge    = 30 * 24 * time.Hour
)

func CleanDownload() {
	expiredAt := time.Now().Add(-downloadMaxAge)
	entries, err := os.ReadDir(filesystem.DownloadDir)
	if err != nil {
		slog.Warn("os.ReadDir",
			slog.String("dir", filesystem.DownloadDir),
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		if entry.Name() == ".Trash" {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(expiredAt) {
			continue
		}

		srcPath := filepath.Join(filesystem.DownloadDir, entry.Name())
		dstPath := filepath.Join(filesystem.DownloadTrashDir, entry.Name())
		if go_pkg_filesystem_reader.Exists(dstPath) {
			ext := filepath.Ext(entry.Name())
			dstName := fmt.Sprintf("%s-%d%s",
				entry.Name()[:len(entry.Name())-len(ext)],
				time.Now().Unix(),
				ext)
			dstPath = filepath.Join(filesystem.DownloadTrashDir, dstName)
		}

		if err := os.Rename(srcPath, dstPath); err != nil {
			slog.Warn("os.Rename",
				slog.String("src", srcPath),
				slog.String("error", err.Error()))
		}
	}
}

func CleanDownloadTrash() {
	expiredAt := time.Now().Add(-trashMaxAge)
	entries, err := os.ReadDir(filesystem.DownloadTrashDir)
	if err != nil {
		slog.Warn("os.ReadDir",
			slog.String("dir", filesystem.DownloadDir),
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		path := filepath.Join(filesystem.DownloadTrashDir, entry.Name())
		info, err := entry.Info()
		if err != nil || info.ModTime().After(expiredAt) {
			continue
		}

		if err := os.RemoveAll(path); err != nil {
			slog.Warn("os.RemoveAll",
				slog.String("path", path),
				slog.String("error", err.Error()))
		}
	}
}
