package filesystem

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

type Skill struct {
	Name        string
	Description string
	AbsPath     string
	Path        string
	Content     string
	Body        string
	Hash        string
}

var (
	skillFrontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	skillNameRegex        = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	skillDescRegex        = regexp.MustCompile(`(?m)^description:\s*(.+)$`)
)

func ParseSkill(path string) (*Skill, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs: %w", err)
	}

	contentText, err := go_pkg_filesystem.ReadText(absPath)
	if err != nil {
		return nil, fmt.Errorf("go_pkg_filesystem.ReadText: %w", err)
	}
	content := []byte(contentText)

	hash := fmt.Sprintf("%x", sha256.Sum256(content))
	folderPath := filepath.Dir(path)
	skill := &Skill{
		Name:    filepath.Base(folderPath),
		AbsPath: absPath,
		Path:    folderPath,
		Content: contentText,
		Body:    contentText,
		Hash:    hash,
	}

	header, body, err := extractSkillHeader(content)
	if err != nil {
		return skill, nil
	}
	skill.Body = body

	if matches := skillNameRegex.FindSubmatch(header); matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}
	if matches := skillDescRegex.FindSubmatch(header); matches != nil {
		skill.Description = strings.TrimSpace(string(matches[1]))
	}

	return skill, nil
}

func ParseSkillBytes(absPath, folderPath string, data []byte) *Skill {
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	text := string(data)
	skill := &Skill{
		Name:    filepath.Base(folderPath),
		AbsPath: absPath,
		Path:    folderPath,
		Content: text,
		Body:    text,
		Hash:    hash,
	}
	header, body, err := extractSkillHeader(data)
	if err != nil {
		return skill
	}
	skill.Body = body
	if matches := skillNameRegex.FindSubmatch(header); matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}
	if matches := skillDescRegex.FindSubmatch(header); matches != nil {
		skill.Description = strings.TrimSpace(string(matches[1]))
	}
	return skill
}

func extractSkillHeader(content []byte) ([]byte, string, error) {
	matches := skillFrontmatterRegex.FindSubmatch(content)
	if matches == nil {
		return nil, "", fmt.Errorf("header not found")
	}
	frontmatter := bytes.TrimSpace(matches[1])
	body := strings.TrimSpace(string(matches[2]))
	return frontmatter, body, nil
}
