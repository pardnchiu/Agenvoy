package variant

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentsexecallowskill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func revokeSkillTrust(e *tooltypes.Executor, path string) string {
	sep := string(filepath.Separator)
	skillDir := ""
	for dir := filepath.Dir(path); strings.HasPrefix(dir, filesystem.SkillsDir+sep); dir = filepath.Dir(dir) {
		if go_pkg_filesystem_reader.IsFile(filepath.Join(dir, "SKILL.md")) {
			skillDir = dir
			break
		}
	}
	if skillDir == "" {
		return ""
	}
	docDir := filepath.Join(skillDir, ".doc")
	if path == docDir || strings.HasPrefix(path, docDir+sep) {
		return ""
	}

	names := []string{filepath.Base(skillDir)}
	if one, err := skill.Get(filepath.Join(skillDir, "SKILL.md")); err == nil && one.Name != names[0] {
		names = append(names, one.Name)
	}

	trusted := agentsexecallowskill.Load(e.WorkDir)
	var revoked []string
	for _, name := range names {
		if !trusted[name] {
			continue
		}
		ok, err := agentsexecallowskill.Revoke(name, e.WorkDir)
		if err != nil {
			slog.Warn("[Internal] agents/exec/allow/skill: Revoke",
				slog.String("skill", name),
				slog.String("error", err.Error()))
			continue
		}
		if ok {
			revoked = append(revoked, fmt.Sprintf("%q", name))
		}
	}
	if len(revoked) == 0 {
		return ""
	}
	return fmt.Sprintf("\n%s was trusted; this skill's files changed, so its trust was removed. Tell the user to trust it again in /skill → permission (or WebUI Skills) if needed.", strings.Join(revoked, ", "))
}
