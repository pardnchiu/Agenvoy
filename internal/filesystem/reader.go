package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_parser "github.com/pardnchiu/go-pkg/filesystem/parser"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const MediaMaxBytes = 10 << 20

var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

func ReadFile(ctx context.Context, path string, offset, limit int) (string, error) {
	if IsMedia(path) {
		return TranscribeMedia(ctx, path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		_, chunks, err := go_pkg_filesystem_parser.PDF(ctx, path)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem_parser.PDF", path, err)
		}
		return sliceChunks(chunks, path, offset, limit, "page"), nil

	case ".pptx":
		_, chunks, err := go_pkg_filesystem_parser.PPTX(ctx, path)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem_parser.PPTX", path, err)
		}
		return sliceChunks(chunks, path, offset, limit, "slide"), nil

	case ".docx":
		result, _, err := go_pkg_filesystem_parser.Docx(ctx, path)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem_parser.Docx", path, err)
		}
		return sliceLines(result, path, offset, limit), nil

	case ".csv", ".tsv":
		result, err := go_pkg_filesystem_parser.CSV(ctx, path, offset, limit)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem_parser.CSV", path, err)
		}
		return result, nil
	}
	if imageExts[ext] {
		info, err := os.Stat(path)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "os.Stat", path, err)
		}
		if info.Size() > MediaMaxBytes {
			return "", utilsmsg.New(utilsmsg.Limit, "image", "too large (max %d MiB): %d MiB", MediaMaxBytes>>20, info.Size()>>20)
		}

		result, err := go_pkg_filesystem_parser.Image(ctx, path)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem_parser.Image", path, err)
		}
		return result, nil
	}

	result, err := readPlainText(path)
	if err != nil {
		return "", err
	}
	return sliceLines(result, path, offset, limit), nil
}

func ReadLines(path string, rows []int, contextLines int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case IsMedia(path), imageExts[ext], ext == ".pdf", ext == ".pptx", ext == ".docx", ext == ".csv", ext == ".tsv":
		return "", utilsmsg.New(utilsmsg.Invalid, "lines", "applies to plain-text files; %s is %s, use offset/limit instead", path, ext)
	}

	result, err := readPlainText(path)
	if err != nil {
		return "", err
	}
	return sliceAround(result, path, rows, max(contextLines, 0)), nil
}

func readPlainText(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.Stat", path, err)
	}
	if info.Size() > configs.MAX_DOCUMENT_BYTES {
		return "", utilsmsg.New(utilsmsg.Limit, "file", "too large (max %d MiB): %d MiB", configs.MAX_DOCUMENT_BYTES>>20, info.Size()>>20)
	}

	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadText", path, err)
	}
	// * binary guard: NUL in first 512B (UTF-16 false-positive)
	if strings.IndexByte(result[:min(len(result), 512)], 0) >= 0 {
		return "", utilsmsg.New(utilsmsg.Invalid, path, "is binary file")
	}
	return result, nil
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
	if text == "" {
		return fmt.Sprintf("%s is empty", path)
	}

	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return fmt.Sprintf("%s is empty", path)
	}

	start := max(offset, 1) - 1
	if start >= len(lines) {
		return fmt.Sprintf("offset %d exceeds(%s) %d", offset, path, len(lines))
	}
	end := min(start+limit, len(lines))

	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%d\t%s\n", i+1, lines[i])
	}
	if end < len(lines) {
		fmt.Fprintf(&sb, "[lines %d-%d of %d; call again with offset=%d for more]\n", start+1, end, len(lines), end+1)
	}
	return sb.String()
}

func sliceAround(text, path string, rows []int, contextLines int) string {
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return fmt.Sprintf("%s is empty", path)
	}

	type window struct{ start, end int }
	var windows []window
	var outside []int
	for _, row := range rows {
		if row < 1 || row > len(lines) {
			outside = append(outside, row)
			continue
		}
		windows = append(windows, window{max(row-contextLines, 1), min(row+contextLines, len(lines))})
	}
	if len(windows) == 0 {
		return fmt.Sprintf("rows %v are outside %s, which has %d lines", outside, path, len(lines))
	}

	slices.SortFunc(windows, func(a, b window) int { return a.start - b.start })
	merged := windows[:1]
	for _, w := range windows[1:] {
		last := &merged[len(merged)-1]
		if w.start <= last.end+1 {
			last.end = max(last.end, w.end)
			continue
		}
		merged = append(merged, w)
	}

	var sb strings.Builder
	shown := 0
	for i, w := range merged {
		if i > 0 {
			sb.WriteString("...\n")
		}
		for n := w.start; n <= w.end; n++ {
			fmt.Fprintf(&sb, "%d\t%s\n", n, lines[n-1])
		}
		shown += w.end - w.start + 1
	}
	fmt.Fprintf(&sb, "[%d of %d lines shown around rows %v", shown, len(lines), rows)
	if len(outside) > 0 {
		fmt.Fprintf(&sb, "; rows %v are outside the file", outside)
	}
	sb.WriteString("]\n")
	return sb.String()
}
