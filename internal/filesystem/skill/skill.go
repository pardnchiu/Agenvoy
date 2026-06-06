package skill

import (
	"bytes"
	"crypto/sha256"
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
	Body        string
	Hash        string
}

var (
	skillFrontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	skillNameRegex        = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	skillBodyStripRegex   = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
)

func Get(path string) (*Skill, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath Abs: %w", err)
	}

	str, err := go_pkg_filesystem.ReadText(absPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText [%s]: %w", path, err)
	}

	return ParseBytes(absPath, filepath.Dir(path), []byte(str)), nil
}

func ParseBytes(absPath, folderPath string, data []byte) *Skill {
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	str := string(data)
	skill := &Skill{
		Name:    filepath.Base(folderPath),
		AbsPath: absPath,
		Path:    folderPath,
		Content: str,
		Body:    str,
		Hash:    hash,
	}
	header, body, err := getHeader(data)
	if err != nil {
		return skill
	}

	skill.Body = body
	if matches := skillNameRegex.FindSubmatch(header); matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}
	skill.Description = getDescription(header)
	return skill
}

func getHeader(content []byte) ([]byte, string, error) {
	matches := skillFrontmatterRegex.FindSubmatch(content)
	if matches == nil {
		return nil, "", fmt.Errorf("header not found")
	}
	result := bytes.TrimSpace(matches[1])
	body := strings.TrimSpace(string(matches[2]))
	return result, body, nil
}

func (s *Skill) ResolvedContent() string {
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
