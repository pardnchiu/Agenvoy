package filesystem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

func ParseSkill(path string) (*Skill, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("filepath.Abs: %w", err)
	}

	result, err := go_pkg_filesystem.ReadText(absPath)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText (%s): %w", path, err)
	}
	content := []byte(result)

	hash := fmt.Sprintf("%x", sha256.Sum256(content))
	dir := filepath.Dir(path)
	skill := &Skill{
		Name:    filepath.Base(dir),
		AbsPath: absPath,
		Path:    dir,
		Content: result,
		Body:    result,
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
	skill.Description = parseSkillDescription(header)

	return skill, nil
}

func ParseSkillBytes(absPath, folderPath string, data []byte) *Skill {
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
	header, body, err := extractSkillHeader(data)
	if err != nil {
		return skill
	}

	skill.Body = body
	if matches := skillNameRegex.FindSubmatch(header); matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}
	skill.Description = parseSkillDescription(header)
	return skill
}

func extractSkillHeader(content []byte) ([]byte, string, error) {
	matches := skillFrontmatterRegex.FindSubmatch(content)
	if matches == nil {
		return nil, "", fmt.Errorf("header not found")
	}
	result := bytes.TrimSpace(matches[1])
	body := strings.TrimSpace(string(matches[2]))
	return result, body, nil
}

func parseSkillDescription(header []byte) string {
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

func GetScheduleSkillBody(name string) (string, error) {
	path := ScheduleSkillPath(name)
	if !go_pkg_filesystem_reader.Exists(path) {
		return "", fmt.Errorf("schedule skill (%s) not found", name)
	}

	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem/reader ReadText (%s): %w", path, err)
	}
	return strings.TrimSpace(skillBodyStripRegex.ReplaceAllString(result, "")), nil
}

func TrashScheduleSkill(ctx context.Context, name string) error {
	dir := ScheduleSkillDir(name)
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return nil
	}
	if err := go_pkg_filesystem.CheckDir(ScheduleSkillTrashDir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir (%s): %w", dir, err)
	}

	dst := filepath.Join(ScheduleSkillTrashDir, name)
	if go_pkg_filesystem_reader.Exists(dst) {
		dst = filepath.Join(ScheduleSkillTrashDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	}
	if err := os.Rename(dir, dst); err != nil {
		return err
	}

	RunTrashCommitSkillDir(ctx, name)

	return nil
}
