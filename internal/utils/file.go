package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

func ReadFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func WriteFile(path, content string, permission os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}
	// * ensure atomic write:
	// * pre-save data as temp
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), permission); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}
	// * rename temp to target
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("os.Rename: %w", err)
	}
	return nil
}
