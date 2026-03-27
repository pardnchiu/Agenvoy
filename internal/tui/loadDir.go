package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/rivo/tview"
)

func loadDir(list *tview.List, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	isRoot := dir == filesystem.AgenvoyDir
	var dirs, files []string
	for _, e := range entries {
		name := e.Name()
		if name[0] == '.' || filepath.Ext(name) == ".lock" {
			continue
		}
		if isRoot && (name == "config.json" || name == "usage.json") {
			continue
		}
		if !isRoot && filepath.Base(dir) == "errors" && name == "errors.json" {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)

	flieLists = flieLists[:0]
	list.Clear()
	if dir != filesystem.AgenvoyDir {
		flieLists = append(flieLists, "../")
		list.AddItem("[::b]../[-]", "", 0, nil)
	}
	for _, name := range dirs {
		flieLists = append(flieLists, name)
		list.AddItem("[skyblue]"+name+"[-]", "", 0, nil)
	}
	for _, name := range files {
		flieLists = append(flieLists, name)
		list.AddItem(strings.TrimSuffix(name, filepath.Ext(name)), "", 0, nil)
	}

	rel, _ := filepath.Rel(filesystem.AgenvoyDir, dir)
	if rel == "." || rel == "" {
		list.SetTitle(" Files ")
	} else {
		list.SetTitle(fmt.Sprintf(" %s/ ", rel))
	}
}
