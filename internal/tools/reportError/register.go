package reportError

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	tailWindowBytes = int64(8 * 1024 * 1024)
	maxReportLines  = 500
	reportEndpoint  = "https://report.agenvoy.com"
	uploadTimeout   = 30 * time.Second
)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "report_error",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Collect daemon-side failures: scan daemon.log for WARN/ERROR lines in the last `h` hours and, when any are found, upload them to report.agenvoy.com (empty result uploads nothing). Returns the collected lines plus an upload-status line. Call ONLY when the current user input explicitly contains 'report error' or 'report_error' — never infer it from generic phrasing like 'check errors / what went wrong / 排錯', which route to list_log instead.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"h": map[string]any{
					"type":        "integer",
					"description": "Look-back window in hours: keep only records whose timestamp is within the last `h` hours from now. Default 1, minimum 1, maximum 168 (one week). Lines without a parseable timestamp inherit the previous line's time (multi-line entries are kept together).",
					"default":     1,
					"minimum":     1,
					"maximum":     168,
				},
			},
		},
		Handler: handler,
	})
}

func handler(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
	var params struct {
		H int `json:"h"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if params.H <= 0 {
		params.H = 1
	}
	params.H = min(params.H, 168)
	cutoff := time.Now().Add(-time.Duration(params.H) * time.Hour)

	path := filepath.Join(filesystem.AgenvoyDir, "daemon.log")
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("Stat: %w", err)
	}
	size := st.Size()

	offset := int64(0)
	readSize := size
	if size > tailWindowBytes {
		offset = size - tailWindowBytes
		readSize = tailWindowBytes
	}

	buf := make([]byte, readSize)
	if readSize > 0 {
		if _, err := f.ReadAt(buf, offset); err != nil {
			return "", fmt.Errorf("ReadAt: %w", err)
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

	lines := strings.Split(text, "\n")
	collected := make([]string, 0, maxReportLines)
	var lastTime time.Time
	var haveTime bool
	for _, line := range lines {
		if line == "" {
			continue
		}
		if t, ok := parseLineTime(line); ok {
			lastTime = t
			haveTime = true
		}
		if haveTime && lastTime.Before(cutoff) {
			continue
		}
		if !isWarnOrError(line) {
			continue
		}
		collected = append(collected, line)
	}

	truncated := false
	if len(collected) > maxReportLines {
		collected = collected[len(collected)-maxReportLines:]
		truncated = true
	}

	if len(collected) == 0 {
		return fmt.Sprintf("(no WARN/ERROR lines in the last %dh)", params.H), nil
	}
	out := strings.Join(collected, "\n")
	if truncated {
		out += fmt.Sprintf("\n(showing last %d; more matched — narrow `h`)", maxReportLines)
	}

	if err := uploadReport(ctx, out); err != nil {
		return out + fmt.Sprintf("\n\n(upload to %s failed: %v)", reportEndpoint, err), nil
	}
	return out + fmt.Sprintf("\n\n(uploaded %d lines to %s)", len(collected), reportEndpoint), nil
}

func uploadReport(ctx context.Context, body string) error {
	client := &http.Client{Timeout: uploadTimeout}
	if _, _, err := go_pkg_http.POST[string](ctx, client, reportEndpoint, nil, map[string]any{"report": body}, "json"); err != nil {
		return fmt.Errorf("POST: %w", err)
	}
	return nil
}

func isWarnOrError(line string) bool {
	return strings.Contains(line, "WARN") || strings.Contains(line, "ERROR")
}

func parseLineTime(line string) (time.Time, bool) {
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
