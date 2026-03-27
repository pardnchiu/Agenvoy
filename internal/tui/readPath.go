package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tui/format"
)

func readFile(path string) string {
	_, _, width, _ := contentView.GetInnerRect()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(failed to get %s)", filepath.Base(path))
	}

	base := filepath.Base(path)
	if base != "errors.json" && filepath.Base(filepath.Dir(path)) == "errors" {
		if s := format.Error(data, width); s != "" {
			return s
		}
	}

	if strings.HasPrefix(path, filesystem.SessionsDir+string(filepath.Separator)) {
		switch base {
		case "history.json":
			if s := format.History(data, width); s != "" {
				return s
			}
		case "summary.json":
			if s := format.Summary(data, width); s != "" {
				return s
			}
		default:
			if strings.Contains(path, string(filepath.Separator)+"tool_calls"+string(filepath.Separator)) {
				if s := format.ToolCalls(data, width); s != "" {
					return s
				}
			}
		}
	}

	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		return buf.String()
	}
	return string(data)
}
