package record

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

const (
	maxSize    = 1 << 20
	trimToSize = 768 << 10
)

func TrimLog() error {
	stat, err := os.Stat(filesystem.DaemonLogPath)
	if err != nil {
		return fmt.Errorf("os.Stat [%s]: %w", filesystem.DaemonLogPath, err)
	}
	if stat.Size() <= maxSize {
		return nil
	}

	content, err := go_pkg_filesystem.ReadText(filesystem.DaemonLogPath)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem.ReadText [%s]: %w", filesystem.DaemonLogPath, err)
	}

	raw := []byte(content)
	if int64(len(raw)) <= maxSize {
		return nil
	}

	result := max(len(raw)-trimToSize, 0)
	for result < len(raw) && raw[result] != '\n' {
		result++
	}
	if result < len(raw) {
		result++
	}

	if err := go_pkg_filesystem.WriteText(filesystem.DaemonLogPath, string(raw[result:])); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem.WriteText [%s]: %w", filesystem.DaemonLogPath, err)
	}
	return nil
}

func GetLog(maxBytes int64, startFrom time.Time) ([]string, error) {
	lines, err := tailLogs(maxBytes)
	if err != nil {
		return nil, err
	}

	newLines := make([]string, 0, len(lines))
	var lastTime time.Time
	var haveTime bool
	for _, line := range lines {
		if line == "" {
			continue
		}
		if t, ok := parseLog(line); ok {
			lastTime = t
			haveTime = true
		}
		if haveTime && lastTime.Before(startFrom) {
			continue
		}
		if !strings.Contains(line, "WARN") && !strings.Contains(line, "ERROR") {
			continue
		}
		newLines = append(newLines, line)
	}
	return newLines, nil
}

func tailLogs(maxBytes int64) ([]string, error) {
	file, err := os.Open(filesystem.DaemonLogPath)
	if err != nil {
		return nil, fmt.Errorf("os.Open [%s]: %w", filesystem.DaemonLogPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("os.File.Stat [%s]: %w", filesystem.DaemonLogPath, err)
	}

	size := stat.Size()
	readSize := size
	offset := int64(0)
	if maxBytes > 0 && size > maxBytes {
		offset = size - maxBytes
		readSize = maxBytes
	}

	raw := make([]byte, readSize)
	if readSize > 0 {
		if _, err := file.ReadAt(raw, offset); err != nil {
			return nil, fmt.Errorf("os.File.ReadAt [%s]: %w", filesystem.DaemonLogPath, err)
		}
	}

	content := string(raw)
	if offset > 0 {
		if i := strings.IndexByte(content, '\n'); i >= 0 {
			content = content[i+1:]
		} else {
			content = ""
		}
	}
	return strings.Split(content, "\n"), nil
}

func parseLog(line string) (time.Time, bool) {
	if rest, ok := strings.CutPrefix(line, "time="); ok {
		ts, _, _ := strings.Cut(rest, " ")
		if t, err := time.Parse("2006-01-02T15:04:05.000-07:00", ts); err == nil {
			return t, true
		}
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return t, true
		}
	}
	if len(line) >= 19 {
		if t, err := time.ParseInLocation("2006/01/02 15:04:05", line[:19], time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
