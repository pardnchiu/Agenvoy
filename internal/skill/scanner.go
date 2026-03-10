package skill

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".opencode", "skills"),
		filepath.Join(home, ".openai", "skills"),
		filepath.Join(home, ".codex", "skills"),
		"/mnt/skills/public",
		"/mnt/skills/user",
		"/mnt/skills/examples",
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

	// * concurrent scan path list
	var wg sync.WaitGroup
	skillChan := make(chan *Skill, 100)
	errChan := make(chan error, len(s.paths))
	for _, path := range s.paths {
		wg.Add(1)

		go func(dir string) {
			defer wg.Done()
			if err := s.scan(dir, skillChan); err != nil {
				errChan <- fmt.Errorf("s.scan %s: %w", dir, err)
			}
		}(path)
	}

	go func() {
		wg.Wait()
		close(skillChan)
		close(errChan)
	}()

	for skill := range skillChan {
		if _, ok := list.ByName[skill.Name]; ok {
			continue
		}
		list.ByName[skill.Name] = skill
		list.ByPath[skill.AbsPath] = skill
	}

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
		slog.Warn("scan error",
			slog.String("error", err.Error()))
	}

	s.mu.Lock()
	s.Skills = list
	s.mu.Unlock()
}

func (s *SkillScanner) scan(root string, skillChan chan<- *Skill) error {
	// * path not exists
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name()[0] == '.' {
			continue
		}

		// ~/.claude/skills/
		// └── {skill_name}/
		//     └── SKILL.md
		path := filepath.Join(root, e.Name(), "SKILL.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		skill, err := parser(path)
		if err != nil {
			slog.Warn("failed to parse skill",
				slog.String("path", path),
				slog.String("error", err.Error()))
			continue
		}
		skillChan <- skill
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
	names := make([]string, 0, len(s.Skills.ByName))
	for name := range s.Skills.ByName {
		names = append(names, strings.TrimSpace(name))
	}
	return names
}
