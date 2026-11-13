package file

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	defaultSearchLimit = 1 << 8 // 256
	maxMatchTextBytes  = 1 << 9
	maxEntryLines      = 5
)

type searchFileEntry struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	Lines []int  `json:"lines"`
}

type searchPage struct {
	offset   int
	returned int
	total    int
	byBytes  bool
}

func (p searchPage) notice(unit string) string {
	next := p.offset + p.returned
	if next >= p.total {
		if p.offset == 0 {
			return ""
		}
		return fmt.Sprintf("\n[%s %d-%d of %d; last page]", unit, p.offset+1, next, p.total)
	}
	reason := "limit"
	if p.byBytes {
		reason = fmt.Sprintf("%d KiB cap", maxFindResultBytes>>10)
	}
	return fmt.Sprintf("\n[%s %d-%d of %d, stopped at %s; call again with offset=%d for the next page]",
		unit, p.offset+1, next, p.total, reason, next)
}

func matchLines(lines []go_pkg_filesystem_reader.Line) []int {
	list := make([]int, 0, maxEntryLines)
	for _, l := range lines {
		if l.Context {
			continue
		}
		list = append(list, l.Line)
		if len(list) == maxEntryLines {
			break
		}
	}
	return list
}

func countMatches(lines []go_pkg_filesystem_reader.Line) int {
	n := 0
	for _, l := range lines {
		if !l.Context {
			n++
		}
	}
	return n
}

func searchBatch(ctx context.Context, e *tooltypes.Executor, queries []findQuery, output string, offset, limit, contextLines int, multiline bool) (string, error) {
	switch output {
	case "files":
		contextLines = 0
	case "content":
		contextLines = max(contextLines, 0)
	default:
		return "", utilsmsg.New(utilsmsg.Invalid, "output", "unknown %q; available: files, content", output)
	}

	seen := make(map[string]struct{})
	var merged []go_pkg_filesystem_reader.File
	for _, q := range queries {
		matches, err := searchOne(ctx, e, q.Dir, q.Pattern, q.FilePattern, contextLines, multiline)
		if err != nil {
			return "", err
		}
		for _, m := range matches {
			if _, ok := seen[m.Path]; ok {
				continue
			}
			seen[m.Path] = struct{}{}
			merged = append(merged, m)
		}
	}

	if len(merged) == 0 {
		return "no files found", nil
	}

	slices.SortFunc(merged, func(a, b go_pkg_filesystem_reader.File) int {
		return strings.Compare(a.Path, b.Path)
	})

	offset = max(offset, 0)
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	if output == "files" {
		return searchFilesPage(merged, offset, limit)
	}
	return searchContentPage(merged, offset, limit, contextLines)
}

func searchFilesPage(merged []go_pkg_filesystem_reader.File, offset, limit int) (string, error) {
	page := searchPage{offset: offset, total: len(merged)}
	if offset >= page.total {
		return fmt.Sprintf("offset %d exceeds %d matching files", offset, page.total), nil
	}

	list := make([]searchFileEntry, 0, min(limit, page.total-offset))
	left := maxFindResultBytes
	for _, f := range merged[offset:min(offset+limit, page.total)] {
		entry := searchFileEntry{Path: f.Path, Count: countMatches(f.Matches), Lines: matchLines(f.Matches)}
		raw, _ := json.Marshal(entry)
		if len(raw) > left && len(list) > 0 {
			page.byBytes = true
			break
		}
		left -= len(raw)
		list = append(list, entry)
	}
	page.returned = len(list)

	raw, err := json.Marshal(list)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw) + page.notice("files"), nil
}

func truncateMatchText(line go_pkg_filesystem_reader.Line) go_pkg_filesystem_reader.Line {
	if len(line.Text) > maxMatchTextBytes {
		line.Text = strings.ToValidUTF8(line.Text[:maxMatchTextBytes], "") + "..."
	}
	return line
}

func lineBytes(line go_pkg_filesystem_reader.Line) int {
	raw, _ := json.Marshal(line)
	return len(raw)
}

func searchContentPage(merged []go_pkg_filesystem_reader.File, offset, limit, contextLines int) (string, error) {
	page := searchPage{offset: offset}
	for _, f := range merged {
		page.total += countMatches(f.Matches)
	}
	if offset >= page.total {
		return fmt.Sprintf("offset %d exceeds %d matching lines", offset, page.total), nil
	}

	var list []go_pkg_filesystem_reader.File
	left := maxFindResultBytes
	skip := offset
	stopped := false
	for _, f := range merged {
		entry := f
		entry.Matches = nil
		header, _ := json.Marshal(entry)
		overhead := len(header)

		var pending []go_pkg_filesystem_reader.Line
		lastIncluded := 0
		for _, l := range f.Matches {
			l = truncateMatchText(l)
			if l.Context {
				if lastIncluded > 0 && l.Line <= lastIncluded+contextLines {
					left -= lineBytes(l)
					entry.Matches = append(entry.Matches, l)
				} else {
					pending = append(pending, l)
				}
				continue
			}

			if skip > 0 {
				skip--
				pending = pending[:0]
				lastIncluded = 0
				continue
			}
			if page.returned >= limit {
				stopped = true
				break
			}

			var before []go_pkg_filesystem_reader.Line
			cost := lineBytes(l) + overhead
			for _, p := range pending {
				if p.Line >= l.Line-contextLines {
					before = append(before, p)
					cost += lineBytes(p)
				}
			}
			if cost > left && page.returned > 0 {
				page.byBytes = true
				stopped = true
				break
			}

			left -= cost
			overhead = 0
			entry.Matches = append(entry.Matches, before...)
			entry.Matches = append(entry.Matches, l)
			pending = pending[:0]
			lastIncluded = l.Line
			page.returned++
		}
		if countMatches(entry.Matches) > 0 {
			list = append(list, entry)
		}
		if stopped {
			break
		}
	}

	raw, err := json.Marshal(list)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw) + page.notice("lines"), nil
}

func searchOne(ctx context.Context, e *tooltypes.Executor, dir, pattern, filePattern string, contextLines int, multiline bool) ([]go_pkg_filesystem_reader.File, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, utilsmsg.New(utilsmsg.Invalid, "pattern", "required")
	}

	dir = strings.TrimSpace(dir)
	absPath, err := boundary.ResolveRead(e.SessionID, e.WorkDir, dir)
	if err != nil {
		return nil, utilsmsg.Trace("boundary.ResolveRead", "", err)
	}

	var filePatterns []string
	if filePattern != "" {
		filePatterns = strings.Split(filepath.ToSlash(filePattern), "/")
	}
	matches, err := go_pkg_filesystem_reader.SearchFiles(absPath, pattern, filePatterns, 0,
		go_pkg_filesystem_reader.ListOption{
			SkipExcluded:    true,
			SkipDenied:      true,
			IgnoreWalkError: true,
			Context:         contextLines,
			Multiline:       multiline,
		})
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem_reader.SearchFiles", "", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	for i, m := range matches {
		if rel, err := filepath.Rel(absPath, m.Path); err == nil {
			matches[i].Path = rel
		}
	}
	return matches, nil
}
