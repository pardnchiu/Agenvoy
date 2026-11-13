package file

import (
	"context"
	"os"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func listBatch(ctx context.Context, e *tooltypes.Executor, queries []findQuery) (string, error) {
	out := make(map[string]any, len(queries))
	budget := newSizeBudget("alphabetical by path")
	for _, q := range queries {
		files, err := listOne(ctx, e, q.Dir, q.Recursive)
		if err != nil {
			out[q.Dir] = "error: " + err.Error()
			continue
		}
		out[q.Dir] = budget.take(files)
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	raw, err := utils.MarshalPlain(out)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "utils.MarshalPlain", "", err)
	}
	return string(raw) + budget.notice(), nil
}

func listOne(ctx context.Context, e *tooltypes.Executor, dir string, recursive bool) ([]go_pkg_filesystem_reader.File, error) {
	dir = strings.TrimSpace(dir)
	absPath, err := boundary.ResolveRead(e.SessionID, e.WorkDir, dir)
	if err != nil {
		return nil, utilsmsg.Trace("boundary.ResolveRead", "", err)
	}

	if file, err := os.Open(absPath); err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "os.Open", "", err)
	} else {
		file.Close()
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if recursive {
		files, err := go_pkg_filesystem_reader.WalkFiles(absPath, go_pkg_filesystem_reader.ListOption{
			SkipExcluded:      true,
			SkipDenied:        true,
			IgnoreWalkError:   true,
			IncludeNonRegular: true,
		})
		if err != nil {
			return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem_reader.WalkFiles", "", err)
		}
		return files, nil
	}

	files, err := go_pkg_filesystem_reader.ListAll(absPath, go_pkg_filesystem_reader.ListOption{SkipExcluded: true})
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem_reader.ListAll", "", err)
	}
	return files, nil
}
