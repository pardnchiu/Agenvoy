package fileReader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

func ListFiles(absPath string, recursive bool) (string, error) {
	results := []file{}

	if recursive {
		walked, err := go_utils_filesystem.WalkFiles(absPath, go_utils_filesystem.ListOption{
			SkipExcluded:      true,
			IgnoreWalkError:   true,
			IncludeNonRegular: true,
		})
		if err != nil {
			return "", fmt.Errorf("go_utils_filesystem.WalkFiles: %w", err)
		}
		for _, rel := range walked {
			full := filepath.Join(absPath, rel)
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
	} else {
		entries, err := go_utils_filesystem.ListAll(absPath, go_utils_filesystem.ListOption{SkipExcluded: true})
		if err != nil {
			return "", fmt.Errorf("go_utils_filesystem.ListAll: %w", err)
		}
		for _, entry := range entries {
			full := filepath.Join(absPath, entry.Name())
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
	}

	data, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(data), nil
}
