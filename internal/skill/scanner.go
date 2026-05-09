package skill

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type SkillScanner struct {
	paths  []string
	Skills *SkillList
	mu     sync.RWMutex
}

type SkillList struct {
	ByName map[string]*Skill
	ByPath map[string]*Skill
	Paths  []string
}

type Skill struct {
	Name        string
	Description string
	AbsPath     string
	Path        string
	Content     string
	Body        string
	Hash        string
}

func NewScanner() *SkillScanner {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	cwd, _ := os.Getwd()
	paths := []string{
		filepath.Join(cwd, ".claude", "skills"),
		filepath.Join(cwd, ".skills"),
		filesystem.SkillsDir,
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".opencode", "skills"),
		filepath.Join(home, ".openai", "skills"),
		filesystem.SystemSkillsDir,
	}

	scanner := &SkillScanner{
		paths: paths,
	}
	scanner.Scan()

	return scanner
}

func (s *SkillScanner) Scan() {
	list := &SkillList{
		ByName: make(map[string]*Skill),
		ByPath: make(map[string]*Skill),
		Paths:  s.paths,
	}

	for _, path := range s.paths {
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

		// ~/.claude/skills/
		// └── {skill_name}/
		//     └── SKILL.md
		path := filepath.Join(root, dir.Name, "SKILL.md")
		if !go_pkg_filesystem_reader.Exists(path) {
			continue
		}

		skill, err := parser(path)
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

func (s *SkillScanner) LoadFS(fsys fs.FS, dir string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		slog.Warn("fs.ReadDir", slog.String("error", err.Error()))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := fmt.Sprintf("%s/%s/SKILL.md", dir, entry.Name())
		data, err := fs.ReadFile(fsys, skillPath)
		if err != nil {
			continue
		}

		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		skill := &Skill{
			Name:    entry.Name(),
			AbsPath: skillPath,
			Path:    fmt.Sprintf("%s/%s", dir, entry.Name()),
			Content: string(data),
			Body:    string(data),
			Hash:    hash,
		}

		header, body, err := extractHeader(data)
		if err == nil {
			skill.Body = body
			if m := nameRegex.FindSubmatch(header); m != nil {
				skill.Name = strings.TrimSpace(string(m[1]))
			}
			if m := descRegex.FindSubmatch(header); m != nil {
				skill.Description = strings.TrimSpace(string(m[1]))
			}
		}

		// * embedded skills is lower than user-defined
		if _, exists := s.Skills.ByName[skill.Name]; exists {
			slog.Info("user-defined exists",
				slog.String("name", skill.Name))
			continue
		}

		s.Skills.ByName[skill.Name] = skill
		s.Skills.ByPath[skill.AbsPath] = skill
		slog.Info("embedded skill loaded", slog.String("name", skill.Name))
	}
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

func (s *SkillScanner) MatchSkillCall(input string) (*Skill, string) {
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

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Skills == nil {
		return nil, input
	}
	skill, ok := s.Skills.ByName[token]
	if !ok {
		return nil, input
	}
	if tail == "" {
		tail = trimmed
	}
	return skill, tail
}
