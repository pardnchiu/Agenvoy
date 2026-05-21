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
)

func parseDescriptionField(header []byte) string {
	lines := strings.Split(string(header), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "description:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		switch rest {
		case "|", "|-", "|+", ">", ">-", ">+":
			fold := rest == ">" || rest == ">-" || rest == ">+"
			var sb strings.Builder
			for j := i + 1; j < len(lines); j++ {
				l := lines[j]
				if strings.TrimSpace(l) == "" {
					if sb.Len() > 0 && !fold {
						sb.WriteString("\n")
					}
					continue
				}
				if !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
					break
				}
				if sb.Len() > 0 {
					if fold {
						sb.WriteString(" ")
					} else {
						sb.WriteString("\n")
					}
				}
				sb.WriteString(strings.TrimSpace(l))
			}
			return strings.TrimSpace(sb.String())
		default:
			return rest
		}
	}
	return ""
}

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
	skill.Description = parseDescriptionField(header)

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
	skill.Description = parseDescriptionField(header)
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
