package fileReader

import (
	"encoding/json"
	"fmt"
	"os"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

type file struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func GlobFiles(absPath, pattern string) (string, error) {
	matches, err := go_pkg_filesystem_reader.GlobFiles(absPath, pattern)
	if err != nil {
		return "", fmt.Errorf("go_pkg_filesystem_reader.GlobFiles: %w", err)
	}
	results := []file{}
	for _, full := range matches {
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		results = append(results, file{
			Name:    info.Name(),
			Path:    full,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}
	data, _ := json.Marshal(results)
	return string(data), nil
}
