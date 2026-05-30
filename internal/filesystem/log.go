package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tailDaemonLog reads up to maxBytes from the tail of daemon.log, drops the
// leading partial line when the window starts mid-file, and returns complete
// lines in file order (oldest first). maxBytes <= 0 reads the whole file.
func tailDaemonLog(maxBytes int64) ([]string, error) {
	// * os.Open retained: tail read via ReadAt over a trailing byte window
	path := filepath.Join(AgenvoyDir, "daemon.log")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("Stat: %w", err)
	}
	size := st.Size()

	offset := int64(0)
	readSize := size
	if maxBytes > 0 && size > maxBytes {
		offset = size - maxBytes
		readSize = maxBytes
	}

	buf := make([]byte, readSize)
	if readSize > 0 {
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("ReadAt: %w", err)
		}
	}

	text := string(buf)
	if offset > 0 {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		} else {
			text = ""
		}
	}
	return strings.Split(text, "\n"), nil
}

// ScanDaemonLogSince tails up to maxBytes of daemon.log and returns WARN/ERROR
// lines whose timestamp is at or after cutoff, in file order. Lines without a
// parseable timestamp inherit the previous line's time so multi-line entries
// stay together. Callers apply their own cap / further filtering.
func ScanDaemonLogSince(maxBytes int64, cutoff time.Time) ([]string, error) {
	lines, err := tailDaemonLog(maxBytes)
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
		if t, ok := parseLogLineTime(line); ok {
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

func parseLogLineTime(line string) (time.Time, bool) {
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
