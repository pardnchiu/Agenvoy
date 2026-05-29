package filesystem

import (
	"fmt"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

func ReadConfig() (map[string]any, error) {
	dic, err := go_pkg_filesystem.ReadJSON[map[string]any](ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON: %w", err)
	}
	if dic == nil {
		dic = map[string]any{}
	}
	return dic, nil
}

func WriteConfig(m map[string]any) error {
	if err := go_pkg_filesystem.WriteJSON(ConfigPath, m, false); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON: %w", err)
	}
	return nil
}
