package tui

import (
	"fmt"
	"os"
	"path/filepath"
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
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
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
	})
}
