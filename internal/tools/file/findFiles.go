package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const maxFindResultBytes = 1 << 17

type sizeBudget struct {
	left    int
	total   int
	dropped int
	order   string
}

func newSizeBudget(order string) *sizeBudget {
	return &sizeBudget{left: maxFindResultBytes, order: order}
}

func entrySize(file go_pkg_filesystem_reader.File) int {
	raw, _ := json.Marshal(file)
	return len(raw)
}

func (b *sizeBudget) take(list []go_pkg_filesystem_reader.File) []go_pkg_filesystem_reader.File {
	b.total += len(list)
	for i := range list {
		size := entrySize(list[i])
		if size > b.left {
			b.dropped += len(list) - i
			return list[:i]
		}
		b.left -= size
	}
	return list
}

func (b *sizeBudget) notice() string {
	if b.dropped == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n[partial result: %d of %d matching entries returned, %s; %d omitted to stay under %d KiB",
		b.total-b.dropped, b.total, b.order, b.dropped, maxFindResultBytes>>10)
	sb.WriteString(". What is here is accurate, only incomplete. To see the rest, narrow dir, make pattern more specific, or tighten file_pattern — re-running this query unchanged truncates identically.]")
	return sb.String()
}

type findQuery struct {
	Dir         string `json:"dir"`
	Pattern     string `json:"pattern"`
	FilePattern string `json:"file_pattern"`
	Recursive   bool   `json:"recursive"`
}

func registFindFiles() {
	toolregister.Regist(toolregister.Def{
		Name:        "find_files",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Locates files: the entries of a directory (list), the paths matching a filename glob (glob), or the files whose contents match an RE2 regex (search, paged).
Use for "find the file", "what's in this folder", "which file contains this", "where is X defined", and for list_files / glob_files / search_files / grep; a path you are unsure of comes from here, never from a guess.
Reading contents → read_files; past versions of a file → file_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "glob", "search"},
					"description": "list: entries of each dir, alphabetical. glob: paths matching pattern, most recently modified first. search: files whose contents match pattern, alphabetical, paged. Omitted: any query with pattern + file_pattern → search, pattern alone → glob, no pattern → list; set it explicitly to search without file_pattern.",
					"default":     "list",
				},
				"output": map[string]any{
					"type":        "string",
					"enum":        []string{"files", "content"},
					"description": "mode=search only. files: each matching path with its match count and the rows of its first 5 matches; start here and pass those rows to read_files(around) when the query was precise. content: the matching lines with their text, for when rows alone cannot tell which match matters.",
					"default":     "files",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "mode=search only: entries to skip (files for output=files, matching lines for output=content). Use the offset the page notice gives and keep queries and output unchanged between pages.",
					"default":     0,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "mode=search only: entries per page (files or matching lines, per output).",
					"default":     defaultSearchLimit,
				},
				"multiline": map[string]any{
					"type":        "boolean",
					"description": "mode=search only: match the regex against whole files so '.' crosses newlines ('^'/'$' still anchor lines), for spans like 'type \\w+ struct \\{.*?Name'. Every row a match spans is reported as a matching line.",
					"default":     false,
				},
				"context": map[string]any{
					"type":        "integer",
					"description": "mode=search, output=content: lines shown before and after each match, marked context=true; they do not count toward offset/limit.",
					"default":     0,
				},
				"queries": map[string]any{
					"type":        "array",
					"description": "Every directory and pattern in one call rather than repeated calls; glob and search merge and deduplicate matches across queries. Every query needs pattern when mode is glob or search.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"dir": map[string]any{
								"type":        "string",
								"description": "Directory to work in, relative to the work directory or absolute: '.', '~/Desktop', '/abs/path'.",
								"default":     ".",
							},
							"pattern": map[string]any{
								"type":        "string",
								"description": "mode=glob: filename glob relative to dir, e.g. '**/*.go', '*.md'; no leading '/' or '~', and it must contain a literal character (all-wildcard is rejected). mode=search: RE2 regex matched per line (whole file with multiline), e.g. 'func\\s+\\w+Handler', 'TODO:'.",
							},
							"file_pattern": map[string]any{
								"type":        "string",
								"description": "mode=search: glob narrowing which files under dir are scanned, e.g. '**/*.go', 'configs/**/*.json'.",
								"default":     "**/*",
							},
							"recursive": map[string]any{
								"type":        "boolean",
								"description": "mode=list: walk the subtree instead of immediate children.",
								"default":     false,
							},
						},
					},
				},
			},
			"required": []string{
				"queries",
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}

			var params struct {
				Mode      string      `json:"mode"`
				Output    string      `json:"output"`
				Offset    int         `json:"offset"`
				Limit     int         `json:"limit"`
				Context   int         `json:"context"`
				Multiline bool        `json:"multiline"`
				Queries   []findQuery `json:"queries"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if len(params.Queries) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "queries", "required")
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				mode = inferMode(params.Queries)
			}

			switch mode {
			case "list":
				return listBatch(ctx, e, params.Queries)
			case "glob":
				if err := requirePattern(params.Queries, mode); err != nil {
					return "", err
				}
				return globBatch(ctx, e, params.Queries)
			case "search":
				if err := requirePattern(params.Queries, mode); err != nil {
					return "", err
				}
				output := strings.ToLower(strings.TrimSpace(params.Output))
				if output == "" {
					output = "files"
				}
				return searchBatch(ctx, e, params.Queries, output, params.Offset, params.Limit, params.Context, params.Multiline)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: list, glob, search", mode)
		},
	})
}

func inferMode(queries []findQuery) string {
	mode := "list"
	for _, q := range queries {
		if strings.TrimSpace(q.Pattern) == "" {
			continue
		}
		if strings.TrimSpace(q.FilePattern) != "" {
			return "search"
		}
		mode = "glob"
	}
	return mode
}

func requirePattern(queries []findQuery, mode string) error {
	for _, q := range queries {
		if strings.TrimSpace(q.Pattern) == "" {
			return utilsmsg.New(utilsmsg.Invalid, "pattern", "every query needs a pattern when mode=%s", mode)
		}
	}
	return nil
}
