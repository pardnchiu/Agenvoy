package skill

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

type Skill struct {
	Name        string
	Description string
	AbsPath     string
	Path        string
	Content     string
}

var (
	frontRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	nameRegex  = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	bodyRegex  = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

func Get(path string) (*Skill, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs [%s]: %w", path, err)
	}

	content, err := go_pkg_filesystem.ReadText(absPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem.ReadText [%s]: %w", path, err)
	}

	raw := []byte(content)
	skill := &Skill{
		Name:    filepath.Base(path),
		AbsPath: absPath,
		Path:    path,
		Content: content,
	}
	header, _, err := getFront(raw)
	if err != nil {
		return skill, nil
	}

	if matches := nameRegex.FindSubmatch(header); matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}
	skill.Description = getDescription(header)
	return skill, nil
}

func getFront(content []byte) ([]byte, string, error) {
	matches := frontRegex.FindSubmatch(content)
	if matches == nil {
		return nil, "", fmt.Errorf("header not found")
	}

	front := bytes.TrimSpace(matches[1])
	body := strings.TrimSpace(string(matches[2]))
	return front, body, nil
}

func (s *Skill) Resolved() string {
	content := s.Content
	for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
		resolved := filepath.Join(s.Path, prefix)
		if go_pkg_filesystem_reader.Exists(resolved) {
			content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
		}
	}
	return content
}

func getDescription(header []byte) string {
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
