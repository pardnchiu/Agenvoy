package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	defaultReadLimit     = 1 << 11 // 2048
	defaultAroundContext = 20
)

func registReadFiles() {
	toolregister.Regist(toolregister.Def{
		Name:        "read_files",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Reads files: text (each line prefixed with its row number, which is not in the file), PDF, DOCX, PPTX, CSV/TSV, images, and audio/video as a verbatim transcript.
Use for "read this file", "show me this file", "what does this PDF say", "what is said in this recording", "why did the background / scheduled / chatbot run fail" (~/.config/agenvoy/daemon.log), and for read_file / cat / head / tail; also the required step before edit_file on an existing file. A file this session produced, or an earlier run of the same recurring task, is read back only to diff, continue or cite it, not as reference input.
Locating a file → find_files; opening it in an app → open_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"files": map[string]any{
					"type":        "array",
					"description": "Every file in one call rather than repeated calls. The same path may appear more than once; its results are joined in order.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path": map[string]any{
								"type":        "string",
								"description": "File to read, absolute or relative to the work directory: '/abs/path/foo.go', '~/notes.md', 'relative/file.md'.",
							},
							"offset": map[string]any{
								"type":        "integer",
								"description": "1-based start: line for text, page for PDF, slide for PPTX, row for CSV; for an append-only log such as daemon.log, page from the end. Ignored when around is set.",
								"default":     1,
							},
							"limit": map[string]any{
								"type":        "integer",
								"description": "How many lines (pages, slides, rows) to read from offset; a text file cut short ends with a notice naming the next offset. When find_files already gave the rows, use around instead of a wide range. Ignored when around is set.",
								"default":     defaultReadLimit,
							},
							"around": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "integer"},
								"description": "Plain-text files only: 1-based rows to read around, e.g. [42, 180] from find_files. Each row gets `context` lines on both sides, overlapping windows merge, and a '...' line marks skipped text. When set, offset and limit are ignored.",
							},
							"context": map[string]any{
								"type":        "integer",
								"description": "With around only: lines shown before and after each row; 0 shows the rows alone.",
								"default":     defaultAroundContext,
							},
						},
						"required": []string{
							"path",
						},
					},
				},
			},
			"required": []string{
				"files",
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Files []struct {
					Path    string `json:"path"`
					Offset  int    `json:"offset"`
					Limit   int    `json:"limit"`
					Around  []int  `json:"around"`
					Context *int   `json:"context"`
				} `json:"files"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if len(params.Files) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "files", "required")
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			out := make(map[string]string, len(params.Files))
			for _, f := range params.Files {
				contextLines := defaultAroundContext
				if f.Context != nil {
					contextLines = *f.Context
				}
				content, err := readOne(ctx, e, baseDir, f.Path, f.Offset, f.Limit, f.Around, contextLines)
				if err != nil {
					content = "error: " + err.Error()
				}
				if prev, ok := out[f.Path]; ok {
					content = prev + "\n" + content
				}
				out[f.Path] = content
			}

			result, err := utils.MarshalPlain(out)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "utils.MarshalPlain", "", err)
			}
			return string(result), nil
		},
	})
}

func readOne(ctx context.Context, e *tooltypes.Executor, baseDir, path string, offset, limit int, around []int, contextLines int) (string, error) {
	absPath, err := boundary.ResolveRead(e.SessionID, baseDir, path)
	if err != nil {
		return "", utilsmsg.Trace("boundary.ResolveRead", "", err)
	}
	if absPath == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "path", "required")
	}

	offset = max(offset, 1)
	limit = max(limit, 0)
	if limit == 0 {
		limit = defaultReadLimit
	}

	info, statErr := os.Stat(absPath)
	var content string
	if len(around) > 0 {
		content, err = filesystem.ReadAround(absPath, around, contextLines)
	} else {
		content, err = filesystem.ReadFile(ctx, absPath, offset, limit)
	}
	if err != nil {
		return "", err
	}
	if statErr == nil {
		e.MarkRead(absPath, info.ModTime())
	}
	return content, nil
}
