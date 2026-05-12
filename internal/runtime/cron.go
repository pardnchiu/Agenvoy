package runtime

import (
	"fmt"
	"path/filepath"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type CronEntry struct {
	Expression string `json:"expression"`
	SessionID  string `json:"session_id"`
	Skill      string `json:"skill"`
}

func CronKey(c CronEntry) string {
	return fmt.Sprintf("%s|%s|%s", c.Expression, c.SessionID, c.Skill)
}

func LoadCrons() ([]CronEntry, error) {
	if !go_pkg_filesystem_reader.Exists(filesystem.CronsPath) {
		return nil, nil
	}
	return go_pkg_filesystem.ReadJSON[[]CronEntry](filesystem.CronsPath)
}

func SaveCrons(crons []CronEntry) error {
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(filesystem.CronsPath), true); err != nil {
		return fmt.Errorf("CheckDir: %w", err)
	}
	if crons == nil {
		crons = []CronEntry{}
	}
	return go_pkg_filesystem.WriteJSON(filesystem.CronsPath, crons, true)
}

func AppendCron(c CronEntry) error {
	existing, _ := LoadCrons()
	key := CronKey(c)
	for _, e := range existing {
		if CronKey(e) == key {
			return fmt.Errorf("duplicate cron: %s already scheduled with the same expression", c.Skill)
		}
	}
	existing = append(existing, c)
	return SaveCrons(existing)
}

func RemoveCron(name string) (int, error) {
	existing, err := LoadCrons()
	if err != nil {
		return 0, err
	}
	filtered := make([]CronEntry, 0, len(existing))
	removed := 0
	for _, e := range existing {
		if e.Skill == name {
			removed++
			continue
		}
		filtered = append(filtered, e)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, SaveCrons(filtered)
}

func PatchCron(skillName, newExpression string) (int, error) {
	existing, err := LoadCrons()
	if err != nil {
		return 0, err
	}
	patched := 0
	for i := range existing {
		if existing[i].Skill != skillName {
			continue
		}
		existing[i].Expression = newExpression
		patched++
	}
	if patched == 0 {
		return 0, nil
	}
	return patched, SaveCrons(existing)
}
