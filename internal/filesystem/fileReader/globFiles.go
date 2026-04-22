package fileReader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type file struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func GlobFiles(absPath, pattern string) (string, error) {
	globParts := strings.Split(pattern, "/")

	results := []file{}

	var matches []string
	if slices.Contains(globParts, "**") {
		walked, err := filesystem.WalkFiles(absPath)
		if err != nil {
			return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
		}

		for _, rel := range walked {
			parts := strings.Split(rel, "/")
			if !filesystem.IsMatch(globParts, parts) {
				continue
			}
			matches = append(matches, filepath.Join(absPath, rel))
		}
	} else {
		// * fast path: no `**`, delegate to filepath.Glob
		absPattern := filepath.Join(append([]string{absPath}, globParts...)...)
		files, err := filepath.Glob(absPattern)
		if err != nil {
			return "", fmt.Errorf("filepath.Glob: %w", err)
		}
		matches = files
	}

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

	data, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(data), nil
}
