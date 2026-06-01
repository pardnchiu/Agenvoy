package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_parser "github.com/pardnchiu/go-pkg/filesystem/parser"
)

const (
	maxReadSize  = 1 << 20
	maxImageSize = 10 << 20
)

var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

func ReadFile(ctx context.Context, path string, offset, limit int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		_, chunks, err := go_pkg_filesystem_parser.PDF(ctx, path)
		if err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/parser PDF [%s]: %w", path, err)
		}
		return sliceChunks(chunks, path, offset, limit, "page"), nil

	case ".pptx":
		_, chunks, err := go_pkg_filesystem_parser.PPTX(ctx, path)
		if err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/parser PPTX [%s]: %w", path, err)
		}
		return sliceChunks(chunks, path, offset, limit, "slide"), nil

	case ".docx":
		result, _, err := go_pkg_filesystem_parser.Docx(ctx, path)
		if err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/parser Docx [%s]: %w", path, err)
		}
		return sliceLines(result, path, offset, limit), nil

	case ".csv", ".tsv":
		result, err := go_pkg_filesystem_parser.CSV(ctx, path, offset, limit)
		if err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/parser CSV [%s]: %w", path, err)
		}
		return result, nil
	}
	if imageExts[ext] {
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("os.Stat: %w", err)
		}
		if info.Size() > maxImageSize {
			return "", fmt.Errorf("image too large(10 MB): %d MB", info.Size()/(1<<20))
		}

		result, err := go_pkg_filesystem_parser.Image(ctx, path)
		if err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/parser Image [%s]: %w", path, err)
		}
		return result, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (1 MB): %d MB", info.Size()/(1<<20))
	}

	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText [%s]: %w", path, err)
	}
	// * binary guard: NUL in first 512B (UTF-16 false-positive)
	if strings.IndexByte(result[:min(len(result), 512)], 0) >= 0 {
		return "", fmt.Errorf("%s is binary file", path)
	}
	return sliceLines(result, path, offset, limit), nil
}

func sliceChunks(chunks []go_pkg_filesystem_parser.Chunk, path string, offset, limit int, unit string) string {
	total := len(chunks)
	if total == 0 {
		return fmt.Sprintf("%s is empty", path)
	}

	start := max(offset, 1) - 1
	if start >= total {
		return fmt.Sprintf("offset %d exceeds(%s): %d", offset, path, total)
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

func sliceLines(text, path string, offset, limit int) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
	line := 0
	written := 0
	for scanner.Scan() {
		line++
		if line < offset {
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
		if line == 0 {
			return fmt.Sprintf("%s is empty", path)
		}
		return fmt.Sprintf("offset %d exceeds(%s) %d", offset, path, line)
	}
	return sb.String()
}
