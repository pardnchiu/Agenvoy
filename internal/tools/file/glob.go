package file

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func globBatch(ctx context.Context, e *tooltypes.Executor, queries []findQuery) (string, error) {
	seen := make(map[string]struct{})
	var merged []go_pkg_filesystem_reader.File
	for _, q := range queries {
		matches, err := globOne(ctx, e, q.Dir, q.Pattern)
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

	pathModTime := make(map[string]time.Time, len(merged))
	for _, m := range merged {
		if info, err := os.Stat(m.Path); err == nil {
			pathModTime[m.Path] = info.ModTime()
		}
	}
	slices.SortFunc(merged, func(a, b go_pkg_filesystem_reader.File) int {
		if c := pathModTime[b.Path].Compare(pathModTime[a.Path]); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	budget := newSizeBudget("most recently modified first")
	merged = budget.take(merged)
	raw, err := json.Marshal(merged)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw) + budget.notice(), nil
}

func isCatchAllPattern(pattern string) bool {
	for _, r := range pattern {
		if r != '*' && r != '/' && r != '.' && r != '?' {
			return false
		}
	}
	return true
}

func globOne(ctx context.Context, e *tooltypes.Executor, dir, pattern string) ([]go_pkg_filesystem_reader.File, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, utilsmsg.New(utilsmsg.Invalid, "pattern", "required")
	}
	if isCatchAllPattern(pattern) {
		return nil, utilsmsg.New(utilsmsg.Invalid, "pattern", "%q matches every file and narrows nothing; use mode=list to see what a directory holds, or mode=search to find files by their content", pattern)
	}

	dir = strings.TrimSpace(dir)
	absPath, err := boundary.ResolveRead(e.SessionID, e.WorkDir, dir)
	if err != nil {
		return nil, utilsmsg.Trace("boundary.ResolveRead", "", err)
	}

	matches, err := go_pkg_filesystem_reader.GlobFiles(absPath, pattern)
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem_reader.GlobFiles", "", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return matches, nil
}
