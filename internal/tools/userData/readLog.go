package userData

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/record"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	readLogWindowBytes = int64(1 * 1024 * 1024)
	readLogMaxLines    = 500
)

func registReadLog() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "read_log",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Return recent WARN/ERROR lines from daemon.log (last `h` hours) — the default path for investigating any daemon-side failure. Route here unless the input explicitly says 'report error' / 'report_error' (that goes to report_error instead, which also uploads).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"h": map[string]any{
					"type":        "integer",
					"description": "Look-back window in hours: keep only WARN/ERROR records whose timestamp is within the last `h` hours from now. Default 1, minimum 1, maximum 72 (3 days). Start small and widen if the cause is not in view.",
					"default":     1,
					"minimum":     1,
					"maximum":     72,
				},
				"session": map[string]any{
					"type":        "string",
					"description": "Restrict to lines tagged with this session id (matches `session=<id>` in slog attrs; case-sensitive). Accepts a full id or a prefix like `tg-` / `dc-` / `cli-` to scope to one channel. Blank disables session filtering. Lines without a session attr never match a non-blank value.",
					"default":     "",
				},
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			result, _, err := readDeamonError(args)
			return result, err
		},
	})
}

func readDeamonError(args json.RawMessage) (string, []string, error) {
	var params struct {
		H       int    `json:"h"`
		Session string `json:"session"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", nil, fmt.Errorf("json Unmarshal: %w", err)
	}

	if params.H <= 0 {
		params.H = 1
	}
	params.H = min(params.H, 72)
	cutoff := time.Now().Add(-time.Duration(params.H) * time.Hour)

	lines, err := record.GetLog(readLogWindowBytes, cutoff)
	if err != nil {
		return "", nil, err
	}

	session := strings.TrimSpace(params.Session)
	if session != "" {
		filtered := lines[:0]
		for _, line := range lines {
			if strings.Contains(line, "session="+session) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	if len(lines) == 0 {
		return fmt.Sprintf("(no WARN/ERROR lines in the last %dh)", params.H), nil, nil
	}

	truncated := false
	if len(lines) > readLogMaxLines {
		lines = lines[len(lines)-readLogMaxLines:]
		truncated = true
	}

	result := strings.Join(lines, "\n")
	if truncated {
		result += fmt.Sprintf("\n(showing last %d; more matched — narrow `h`)", readLogMaxLines)
	}
	return result, lines, nil
}
