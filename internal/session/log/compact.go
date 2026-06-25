package sessionLog

import (
	"log/slog"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func RetainExchanges(sessionID string, keptRawContents []string) {
	path := filesystem.ActionLogPath(sessionID)
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil || strings.TrimSpace(text) == "" {
		return
	}

	keepSet := make(map[string]bool, len(keptRawContents))
	for _, c := range keptRawContents {
		keepSet[flatten(strings.TrimSpace(c))] = true
	}

	lines := strings.Split(text, "\n")

	type block struct {
		start    int
		end      int
		userBody string
	}

	var blocks []block
	for i, line := range lines {
		if body, ok := extractKindBody(line, "user"); ok {
			if len(blocks) > 0 {
				blocks[len(blocks)-1].end = i
			}
			blocks = append(blocks, block{start: i, userBody: body})
		}
	}
	if len(blocks) > 0 {
		blocks[len(blocks)-1].end = len(lines)
	}

	removeLine := make([]bool, len(lines))
	for _, b := range blocks {
		if !keepSet[b.userBody] {
			for j := b.start; j < b.end; j++ {
				removeLine[j] = true
			}
		}
	}

	var kept []string
	for i, line := range lines {
		if !removeLine[i] {
			kept = append(kept, line)
		}
	}

	actionLogMu.Lock()
	defer actionLogMu.Unlock()

	if err := go_pkg_filesystem.WriteFile(path, strings.Join(kept, "\n"), 0644); err != nil {
		slog.Warn("compact action.log",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func extractKindBody(line, kind string) (string, bool) {
	_, body, ok := strings.Cut(line, "]["+kind+"] ")
	return body, ok
}
