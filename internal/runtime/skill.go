package runtime

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

var SkillSources = []string{"claude", "codex", "copilot", "opencode", "agents"}

type SkillScanner struct {
	paths  []string
	Skills *SkillList
	mu     sync.RWMutex
}

type SkillList struct {
	ByName map[string]*skill.Skill
	ByPath map[string]*skill.Skill
	Paths  []string
}

func NewSkillScanner() *SkillScanner {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	paths := []string{
		filesystem.SystemSkillsDir,
		filesystem.SystemDesignDir,
		filesystem.SkillsDir,
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".opencode", "skills"),
		filepath.Join(home, ".copilot", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}

	scanner := &SkillScanner{paths: paths}
	scanner.Scan()
	return scanner
}

func (s *SkillScanner) Scan() {
	list := &SkillList{
		ByName: make(map[string]*skill.Skill),
		ByPath: make(map[string]*skill.Skill),
		Paths:  s.paths,
	}

	var disabled []string
	if cfg, err := config.Load(); err == nil {
		disabled = cfg.SkillSourceOff
	}

	for _, path := range s.paths {
		if slices.Contains(disabled, SkillSource(path+"/")) {
			continue
		}
		if err := s.scan(path, list); err != nil {
			slog.Warn("scan error",
				slog.String("path", path),
				slog.String("error", err.Error()))
		}
	}

	s.mu.Lock()
	s.Skills = list
	s.mu.Unlock()
}

func (s *SkillScanner) scan(root string, list *SkillList) error {
	if !go_pkg_filesystem_reader.Exists(root) {
		return nil
	}

	dirs, err := go_pkg_filesystem_reader.ListDirs(root)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if dir.Name[0] == '.' {
			continue
		}

		path := filepath.Join(root, dir.Name, "SKILL.md")
		if !go_pkg_filesystem_reader.Exists(path) {
			continue
		}

		skill, err := skill.Get(path)
		if err != nil {
			slog.Warn("failed to parse skill",
				slog.String("path", path),
				slog.String("error", err.Error()))
			continue
		}
		if _, exists := list.ByName[skill.Name]; exists {
			continue
		}
		list.ByName[skill.Name] = skill
		list.ByPath[skill.AbsPath] = skill
	}

	return nil
}

func (s *SkillScanner) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.Skills.ByName))
	for name := range s.Skills.ByName {
		names = append(names, strings.TrimSpace(name))
	}
	sort.Strings(names)
	return names
}

func (s *SkillScanner) Lookup(name string) *skill.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Skills == nil {
		return nil
	}
	return s.Skills.ByName[name]
}

func MatchSkill(scanner *SkillScanner, input string, excludeSkills ...string) (*skill.Skill, string) {
	if scanner == nil {
		return nil, input
	}
	trimmed := strings.TrimLeft(input, " \t\r\n")
	if !strings.HasPrefix(trimmed, "/") {
		return nil, input
	}
	rest := trimmed[1:]
	token := rest
	tail := ""
	if idx := strings.IndexAny(rest, " \t\r\n"); idx >= 0 {
		token = rest[:idx]
		tail = strings.TrimLeft(rest[idx:], " \t\r\n")
	}
	if token == "" {
		return nil, input
	}
	for _, ex := range excludeSkills {
		if strings.TrimSpace(ex) == token {
			return nil, input
		}
	}
	s := scanner.Lookup(token)
	if s == nil {
		return nil, input
	}
	if tail == "" {
		tail = trimmed
	}
	return s, tail
}

func SkillSource(path string) string {
	if path == "" {
		return ""
	}
	switch {
	case filesystem.SystemSkillsDir != "" && strings.HasPrefix(path, filesystem.SystemSkillsDir+"/"),
		filesystem.SystemDesignDir != "" && strings.HasPrefix(path, filesystem.SystemDesignDir+"/"):
		return "system"
	case filesystem.SkillsDir != "" && strings.HasPrefix(path, filesystem.SkillsDir+"/"):
		return "user"
	case strings.Contains(path, "/.claude/skills/"):
		return "claude"
	case strings.Contains(path, "/.opencode/skills/"):
		return "opencode"
	case strings.Contains(path, "/.codex/skills/"):
		return "codex"
	case strings.Contains(path, "/.copilot/skills/"):
		return "copilot"
	case strings.Contains(path, "/.agents/skills/"):
		return "agents"
	case strings.Contains(path, "/.skills/"):
		return "local"
	case strings.HasPrefix(path, "/mnt/skills/"):
		rest := strings.TrimPrefix(path, "/mnt/skills/")
		if i := strings.IndexByte(rest, '/'); i > 0 {
			return "mnt-" + rest[:i]
		}
	}
	return ""
}
