// * last modified: 26/11/22
package file

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	defaultReadLimit    = 1 << 11 // 2048
	defaultContextLines = 64
)

func registReadFiles() {
	toolregister.Regist(toolregister.Def{
		Name:        "read_files",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Reads files: text (each line prefixed with its row number, which is not in the file), PDF, DOCX, PPTX, CSV/TSV, images, and audio/video as a verbatim transcript.
Use for "read this file", "show me this file", "what does this PDF say", "what is said in this recording", "why did the background / scheduled / chatbot run fail" (~/.config/agenvoy/daemon.log), and for read_file / cat / head / tail; also the required step before write_file on an existing file. A file this session produced, or an earlier run of the same recurring task, is read back only to diff, continue or cite it, not as reference input.
Locating a file → find_files; opening it in an app → open_file; past versions → file_history.`,
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
							// NOTE: offset/limit 用於未知行號的情況
							"offset": map[string]any{
								"type":        "integer",
								"description": "Pairs with limit, for when the rows are not known yet: 1-based start, a line for text, page for PDF, slide for PPTX, row for CSV; for an append-only log such as daemon.log, start near the end.",
								"default":     1,
							},
							"limit": map[string]any{
								"type":        "integer",
								"description": "Pairs with offset: how many lines (pages, slides, rows) to read from it.",
								"default":     defaultReadLimit,
							},
							// NOTE: lines/context_lines 用於已知行號的情況
							"lines": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "integer"},
								"description": "Pairs with context_lines, for when the rows are known, e.g. [42, 180] from find_files' `lines`: plain-text files only, 1-based row numbers to read around instead of a wide offset/limit range. When set, offset and limit are ignored.",
							},
							"context_lines": map[string]any{
								"type":        "integer",
								"description": "Pairs with lines: how many lines to show before and after each row; 0 shows the rows alone.",
								"default":     defaultContextLines,
							},
							"full": map[string]any{
								"type":        "boolean",
								"description": "true returns every row, skipping the relevance filter: before write_file on the file, since parts left out as unrelated would be lost in a mode=write rewrite, or when a filtered read left out rows that are needed.",
								"default":     false,
							},
						},
						"required": []string{"path"},
					},
				},
			},
			"required": []string{"files"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Files []struct {
					Path         string `json:"path"`
					Offset       int    `json:"offset"`
					Limit        int    `json:"limit"`
					Lines        []int  `json:"lines"`
					ContextLines *int   `json:"context_lines"`
					Full         bool   `json:"full"`
				} `json:"files"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}
			if len(params.Files) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "files", "required")
			}

			baseDir := boundary.BaseDir(e.WorkDir)
			contents := make([]string, len(params.Files))
			var wg sync.WaitGroup
			for i, file := range params.Files {
				contextLines := defaultContextLines
				if file.ContextLines != nil {
					contextLines = *file.ContextLines
				}

				content, err := readFile(ctx, e, baseDir, file.Path, file.Offset, file.Limit, file.Lines, contextLines)
				if err != nil {
					contents[i] = "error: " + err.Error()
					continue
				}

				contents[i] = content
				if file.Full {
					continue
				}
				wg.Go(func() {
					contents[i] = checkFileRelevance(ctx, e.SessionID, e.Objective, file.Path, content)
				})
			}
			wg.Wait()

			dic := make(map[string]string, len(params.Files))
			for i, f := range params.Files {
				content := contents[i]
				if prev, ok := dic[f.Path]; ok {
					content = prev + "\n" + content
				}
				dic[f.Path] = content
			}

			raw, err := utils.MarshalPlain(dic)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "utils: MarshalPlain", "", err)
			}
			return string(raw), nil
		},
	})
}

func readFile(ctx context.Context, e *tooltypes.Executor, baseDir, path string, offset, limit int, lines []int, contextLines int) (string, error) {
	absPath, err := boundary.ResolveRead(e.SessionID, baseDir, path)
	if err != nil {
		return "", utilsmsg.Trace("tools/file/boundary: ResolveRead", "", err)
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
	if len(lines) > 0 {
		content, err = filesystem.ReadLines(absPath, lines, contextLines)
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
