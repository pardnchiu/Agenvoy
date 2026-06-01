package record

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func GetLog(maxBytes int64, cutoff time.Time) ([]string, error) {
	lines, err := tailLog(maxBytes)
	if err != nil {
		return nil, err
	}

	collected := make([]string, 0, len(lines))
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
		if haveTime && lastTime.Before(cutoff) {
			continue
		}
		if !strings.Contains(line, "WARN") && !strings.Contains(line, "ERROR") {
			continue
		}
		collected = append(collected, line)
	}
	return collected, nil
}

func tailLog(maxBytes int64) ([]string, error) {
	path := filesystem.DaemonLogPath
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("os Open [%s]: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("os File.Stat [%s]: %w", path, err)
	}
	size := stat.Size()

	offset := int64(0)
	readSize := size
	if maxBytes > 0 && size > maxBytes {
		offset = size - maxBytes
		readSize = maxBytes
	}

	raw := make([]byte, readSize)
	if readSize > 0 {
		if _, err := file.ReadAt(raw, offset); err != nil {
			return nil, fmt.Errorf("os File.ReadAt [%s]: %w", path, err)
		}
	}

	str := string(raw)
	if offset > 0 {
		if i := strings.IndexByte(str, '\n'); i >= 0 {
			str = str[i+1:]
		} else {
			str = ""
		}
	}
	return strings.Split(str, "\n"), nil
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
