package skill

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

// ---
// name: changelog-generate
// description: 從 git diff 輸出生成結構化的 update.md 更新日誌，並自動進行語意化版本控制。當使用者請求生成更新日誌、發布說明，或基於未提交的 git 變更更新文件時使用。
// ---
var (
	headerRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	nameRegex   = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	descRegex   = regexp.MustCompile(`(?m)^description:\s*(.+)$`)
)

func parser(path string) (*Skill, error) {
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

	header, body, err := extractHeader(content)
	if err != nil {
		// * header not exists
		return skill, nil
	}
	skill.Body = body

	matches := nameRegex.FindSubmatch(header)
	if matches != nil {
		skill.Name = strings.TrimSpace(string(matches[1]))
	}

	matches = descRegex.FindSubmatch(header)
	if matches != nil {
		skill.Description = strings.TrimSpace(string(matches[1]))
	}

	return skill, nil
}

func extractHeader(content []byte) ([]byte, string, error) {
	matches := headerRegex.FindSubmatch(content)
	if matches == nil {
		return nil, "", fmt.Errorf("header not found")
	}

	frontmatter := bytes.TrimSpace(matches[1])
	body := strings.TrimSpace(string(matches[2]))

	return frontmatter, body, nil
}
