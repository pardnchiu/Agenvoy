package interactive

import (
	"fmt"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func ListActiveTasks() []string {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}
	var list []string
	for _, dir := range dirs {
		sid := dir.Name
		if strings.HasPrefix(sid, ".") {
			continue
		}
		for _, hash := range ListOnlinePending(sid) {
			title := hash
			if info, ok := LoadPendingInfo(sid, hash); ok && strings.TrimSpace(info.Objective) != "" {
				title = strings.Join(strings.Fields(info.Objective), " ")
			}
			list = append(list, fmt.Sprintf("[running] %s  %s", sid, title))
		}
		for _, leg := range ListRunningBackground(sid) {
			list = append(list, fmt.Sprintf("[background] %s  %s  %s", sid, leg.Description, leg.ID))
		}
	}
	return list
}
