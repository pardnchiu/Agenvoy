package fileReader

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/parser"
)

var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

func ReadFile(ctx context.Context, path string, offset, limit int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".pdf":
		_, chunks, err := parser.PDF(ctx, path)
		if err != nil {
			return "", fmt.Errorf("parser.PDF: %w", err)
		}
		return sliceChunks(chunks, path, offset, limit, "page"), nil
	case ".pptx":
		_, chunks, err := parser.PPTX(ctx, path)
		if err != nil {
			return "", fmt.Errorf("parser.PPTX: %w", err)
		}
		return sliceChunks(chunks, path, offset, limit, "slide"), nil
	case ".docx":
		text, _, err := parser.Docx(ctx, path)
		if err != nil {
			return "", fmt.Errorf("parser.Docx: %w", err)
		}
		return sliceLines(text, path, offset, limit), nil
	case ".csv", ".tsv":
		return getCSV(path, offset, limit)
	}
	if imageExts[ext] {
		return getImage(path)
	}

	// * os.Stat retained: FileInfo.Size() needed for the > 1MB guard before reading
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	fileContent, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", fmt.Errorf("go_pkg_filesystem.ReadText: %w", err)
	}
	if strings.IndexByte(fileContent[:min(len(fileContent), 512)], 0) >= 0 {
		return "", fmt.Errorf("binary file: %s", path)
	}
	return sliceLines(fileContent, path, offset, limit), nil
}

func sliceLines(text, path string, offset, limit int) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(text))
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
	if sb.Len() == 0 {
		if lineNum == 0 {
			return fmt.Sprintf("%s is empty", path)
		}
		return fmt.Sprintf("offset %d exceeds total lines %d in %s", offset, lineNum, path)
	}
	return sb.String()
}

func sliceChunks(chunks []parser.Chunk, path string, offset, limit int, unit string) string {
	total := len(chunks)
	if total == 0 {
		return fmt.Sprintf("%s is empty", path)
	}
	start := max(offset, 1) - 1
	if start >= total {
		return fmt.Sprintf("offset %d exceeds total %ss %d in %s", offset, unit, total, path)
	}
	end := min(start+limit, total)

	var sb strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "--- %s %d/%d ---\n", unit, i+1, total)
		sb.WriteString(chunks[i].Content)
	}
	return sb.String()
}
