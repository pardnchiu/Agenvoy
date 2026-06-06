package exec

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
)

func buildContent(content string, imageInputs []string, fileInputs []string) any {
	if len(imageInputs) == 0 && len(fileInputs) == 0 {
		return content
	}

	parts := []agentTypes.ContentPart{
		{
			Type: "text",
			Text: content,
		},
	}

	for _, path := range fileInputs {
		text, err := go_pkg_filesystem.ReadText(path)
		if err != nil {
			continue
		}
		parts = append(parts, agentTypes.ContentPart{
			Type: "text",
			Text: fmt.Sprintf("---\npath: %s\n---\n%s", filepath.Base(path), text),
		})
	}

	for _, path := range imageInputs {
		b64, err := convertToBase64(path)
		if err != nil {
			continue
		}
		dataURL := "data:image/jpeg;base64," + b64
		parts = append(parts, agentTypes.ContentPart{
			Type:     "image_url",
			ImageURL: &agentTypes.ImageURL{URL: dataURL, Detail: "auto"},
		})
	}
	return parts
}

func GetSession(ctx context.Context, execData ExecData) (*agentTypes.AgentSession, error) {
	scanner := execData.SkillScanner
	if scanner == nil {
		scanner = agents.Scanner()
	}
	trimInput := strings.TrimSpace(execData.Content)
	session := agentTypes.AgentSession{
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	overrideID := strings.TrimSpace(execData.SessionID)
	if overrideID == "" {
		return nil, fmt.Errorf("execData.SessionID is required")
	}
	sessionDir := filesystem.SessionDir(overrideID)
	if !go_pkg_filesystem_reader.IsDir(sessionDir) {
		return nil, fmt.Errorf("session %q does not exist", overrideID)
	}

	oldHistory, maxHistory := sessionHistory.Get(overrideID)
	session.Histories = oldHistory
	session.BaseLen = len(oldHistory)

	session.SystemPrompts = BuildSystemPrompts(execData.WorkDir, execData.ExtraSystemPrompt, scanner, overrideID, execData.AllowAll, execData.ExcludeSkills)
	if summary := summary.GetPrompt(overrideID, OldestMessageTime(maxHistory)); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	session.OldHistories = maxHistory
	session.ToolHistories = []agentTypes.Message{}

	userText := fmt.Sprintf("---\n當前時間: %s\n工作目錄: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), execData.WorkDir, trimInput)
	session.Histories = append(session.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	session.UserInput = agentTypes.Message{
		Role:    "user",
		Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
	}
	SaveUserInputHistory(ctx, overrideID, userText)

	session.ID = overrideID
	return &session, nil
}

var msgTimeRegex = regexp.MustCompile(`當前時間:\s*(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)

func OldestMessageTime(histories []agentTypes.Message) time.Time {
	for _, m := range histories {
		s, ok := m.Content.(string)
		if !ok {
			continue
		}
		if matches := msgTimeRegex.FindStringSubmatch(s); len(matches) > 1 {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", matches[1], time.Local); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func convertToBase64(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("image.Decode: %w", err)
	}

	// * need to be use jpeg before send in claude/gemini model
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("jpeg.Encode: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
