package fileReader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func ListFiles(absPath string, recursive bool) (string, error) {
	results := []file{}

	if recursive {
		walked, err := go_pkg_filesystem_reader.WalkFiles(absPath, go_pkg_filesystem_reader.ListOption{
			SkipExcluded:      true,
			IgnoreWalkError:   true,
			IncludeNonRegular: true,
		})
		if err != nil {
			return "", fmt.Errorf("go_pkg_filesystem_reader.WalkFiles: %w", err)
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
		entries, err := go_pkg_filesystem_reader.ListAll(absPath, go_pkg_filesystem_reader.ListOption{SkipExcluded: true})
		if err != nil {
			return "", fmt.Errorf("go_pkg_filesystem_reader.ListAll: %w", err)
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
