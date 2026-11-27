package tool

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	runtimetooladapter "github.com/pardnchiu/agenvoy/internal/runtime/toolAdapter"
)

type file struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func toolFiles(dir string) []file {
	list := make([]file, 0)
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}

		skip := strings.HasPrefix(entry.Name(), ".") ||
			entry.Type()&fs.ModeSymlink != 0 ||
			filesystem.IsDeniedPath(path) ||
			filesystem.IsSensitivePath(path)
		if entry.IsDir() {
			if skip {
				return fs.SkipDir
			}
			return nil
		}
		if skip {
			return nil
		}

		info, err := entry.Info()
		if err != nil || info.Size() > configs.MAX_DOCUMENT_BYTES {
			return nil
		}

		content, err := go_pkg_filesystem.ReadText(path)
		if err != nil || !utf8.ValidString(content) {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		list = append(list, file{Path: filepath.ToSlash(rel), Content: content})
		return nil
	})

	slices.SortFunc(list, func(a, b file) int {
		switch {
		case a.Path == b.Path:
			return 0
		case a.Path == "tool.json":
			return -1
		case b.Path == "tool.json":
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	return list
}

func toolNames(translator *runtimetooladapter.Translator) []string {
	var names []string
	for _, tool := range translator.GetTools() {
		if fn, ok := tool["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}
