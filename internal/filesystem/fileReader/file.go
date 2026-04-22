package fileReader

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

func ReadFile(path string, offset, limit int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		return getPDF(path, offset, limit)
	}
	if ext == ".csv" || ext == ".tsv" {
		return getCSV(path, offset, limit)
	}
	if imageExts[ext] {
		return "", fmt.Errorf("image file detected, use `read_image` instead")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	fileContent := string(fileBytes)
	if strings.IndexByte(fileContent[:min(len(fileContent), 512)], 0) >= 0 {
		return "", fmt.Errorf("binary file: %s", path)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(fileContent))
	scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
	lineNum := 0
	written := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if written >= limit {
			break
		}
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
		written++
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("bufio.Scanner: %w", err)
	}
	if sb.Len() == 0 {
		if lineNum == 0 {
			return fmt.Sprintf("%s is empty", path), nil
		}
		return fmt.Sprintf("offset %d exceeds total lines %d in %s", offset, lineNum, path), nil
	}
	return sb.String(), nil
}
