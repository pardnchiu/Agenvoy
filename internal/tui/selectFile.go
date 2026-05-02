package tui

import (
	"fmt"
	"path/filepath"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func selectFile(index int, _, _ string, _ rune) {
	if index < 0 || index >= len(flieLists) {
		return
	}

	name := flieLists[index]
	if name == "../" {
		currentDir = filepath.Dir(currentDir)
		go app.QueueUpdateDraw(func() {
			loadDir(filesView, currentDir)
		})
		return
	}

	path := filepath.Join(currentDir, name)
	if !go_pkg_filesystem_reader.Exists(path) {
		return
	}

	if go_pkg_filesystem_reader.IsDir(path) {
		currentDir = path
		go app.QueueUpdateDraw(func() {
			loadDir(filesView, currentDir)
			if filesView.GetItemCount() > 1 {
				filesView.SetCurrentItem(1)
			}
		})
		return
	}

	currentPath = path
	go app.QueueUpdateDraw(func() {
		contentView.SetTitle(fmt.Sprintf(" %s ", name))
		contentView.SetText(readFile(path))
		contentView.ScrollToBeginning()
		viewPages.SwitchToPage("content")
	})
}
