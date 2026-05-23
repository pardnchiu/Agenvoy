package listLog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const tailWindowBytes = int64(1 * 1024 * 1024)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_log",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Tail recent daemon log lines for debugging. Use when the user reports unexplained failures (message not delivered, tool errored, daemon-side warning) or when you need to confirm what the daemon actually observed. When the user says 'check errors / recent failures / 看錯誤 / 排錯', default to level='WARN' — most fail-soft conditions (HTTP 4xx, parse errors, send failures, retries) are recorded at WARN; level='ERROR' alone almost always misses the real cause.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"lines": map[string]any{
					"type":        "integer",
					"description": "Trailing line count to return after filtering. Default 50, max 500. Larger windows cost more tokens; start small and re-call with a tighter filter if the answer is not in view.",
					"default":     50,
					"minimum":     1,
					"maximum":     500,
				},
				"filter": map[string]any{
					"type":        "string",
					"description": "Case-insensitive substring filter applied per line before tailing. Blank returns every line. Pair with a specific identifier (tool name, session id prefix like 'tg-', error keyword) to drop unrelated noise.",
					"default":     "",
				},
				"level": map[string]any{
					"type":        "string",
					"description": "Severity threshold. Blank or INFO returns every line; WARN returns WARN+ERROR (recommended default when user is investigating failures — most issues land at WARN); ERROR returns ERROR only (use sparingly, will hide WARN-level send/parse/HTTP errors). Matches both stdlib log format ('YYYY/MM/DD HH:MM:SS LEVEL ...') and slog format ('level=LEVEL').",
					"default":     "",
					"enum":        []string{"", "INFO", "WARN", "ERROR"},
				},
			},
		},
		Handler: handler,
	})
}

func handler(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
	var params struct {
		Lines  int    `json:"lines"`
		Filter string `json:"filter"`
		Level  string `json:"level"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if params.Lines <= 0 {
		params.Lines = 50
	}
	params.Lines = min(params.Lines, 500)

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

	filter := strings.ToLower(strings.TrimSpace(params.Filter))
	level := strings.ToUpper(strings.TrimSpace(params.Level))

	lines := strings.Split(text, "\n")
	collected := make([]string, 0, params.Lines)
	for i := len(lines) - 1; i >= 0 && len(collected) < params.Lines; i-- {
		line := lines[i]
		if line == "" {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(line), filter) {
			continue
		}
		if !matchLevel(line, level) {
			continue
		}
		collected = append(collected, line)
	}

	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	if len(collected) == 0 {
		return "(no matching lines)", nil
	}
	return strings.Join(collected, "\n"), nil
}

func matchLevel(line, level string) bool {
	switch level {
	case "", "INFO":
		return true
	case "WARN":
		return strings.Contains(line, "WARN") || strings.Contains(line, "ERROR")
	case "ERROR":
		return strings.Contains(line, "ERROR")
	}
	return true
}
