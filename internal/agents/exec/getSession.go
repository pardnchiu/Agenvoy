package exec

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
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

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

type IndexData struct {
	SessionID string `json:"session_id"`
}

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
		text, err := go_utils_filesystem.ReadText(path)
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

func GetSession(execData ExecData) (*agentTypes.AgentSession, error) {
	scanner := execData.SkillScanner
	if scanner == nil {
		scanner = host.Scanner()
	}
	trimInput := strings.TrimSpace(execData.Content)
	session := agentTypes.AgentSession{
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	if overrideID := strings.TrimSpace(execData.SessionID); overrideID != "" {
		sessionDir := filepath.Join(filesystem.SessionsDir, overrideID)
		if !go_utils_filesystem.IsDir(sessionDir) {
			return nil, fmt.Errorf("session %q does not exist", overrideID)
		}

		oldHistory, maxHistory := sessionManager.GetHistory(overrideID)
		session.Histories = oldHistory

		session.SystemPrompts = []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, scanner, overrideID, execData.AllowAll)}}
		if summary := sessionManager.GetSummaryPrompt(overrideID, OldestMessageTime(maxHistory)); summary != "" {
			session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
		}

		session.OldHistories = maxHistory
		session.ToolHistories = []agentTypes.Message{}

		userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), trimInput)
		session.Histories = append(session.Histories, agentTypes.Message{
			Role:    "user",
			Content: userText,
		})
		session.UserInput = agentTypes.Message{
			Role:    "user",
			Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
		}
		SaveUserInputHistory(overrideID, userText)

		session.ID = overrideID
		return &session, nil
	}

	unlock, err := sessionManager.LockConfig()
	if err != nil {
		return nil, fmt.Errorf("lockConfig: %w", err)
	}
	defer unlock()

	var sessionID string
	configExists := go_utils_filesystem.Exists(filesystem.ConfigPath)
	switch {
	case configExists:
		configText, err := go_utils_filesystem.ReadText(filesystem.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("go_utils_filesystem.ReadText: %w", err)
		}
		data := []byte(configText)
		// * config is exist
		var indexData IndexData
		if err := json.Unmarshal(data, &indexData); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}
		if indexData.SessionID == "" {
			newID, err := sessionManager.CreateSession("cli-")
			if err != nil {
				return nil, fmt.Errorf("newSessionID: %w", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				raw = make(map[string]json.RawMessage)
			}
			raw["session_id"], err = json.Marshal(newID)
			if err != nil {
				return nil, fmt.Errorf("json.Marshal: %w", err)
			}
			merged, err := json.Marshal(raw)
			if err != nil {
				return nil, fmt.Errorf("json.Marshal: %w", err)
			}
			if err := go_utils_filesystem.WriteFile(filesystem.ConfigPath, string(merged), 0644); err != nil {
				return nil, fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
			}
			indexData.SessionID = newID
		}
		sessionID = strings.TrimSpace(indexData.SessionID)

		oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
		session.Histories = oldHistory

		session.SystemPrompts = []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, scanner, sessionID, execData.AllowAll)}}
		if summary := sessionManager.GetSummaryPrompt(sessionID, OldestMessageTime(maxHistory)); summary != "" {
			session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
		}

		session.OldHistories = maxHistory
		session.ToolHistories = []agentTypes.Message{}

		userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), trimInput)
		session.Histories = append(session.Histories, agentTypes.Message{
			Role:    "user",
			Content: userText,
		})
		session.UserInput = agentTypes.Message{
			Role:    "user",
			Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
		}
		SaveUserInputHistory(sessionID, userText)

	default:
		// * config is not exist
		sessionID, err := sessionManager.CreateSession("cli-")
		if err != nil {
			return nil, fmt.Errorf("newSessionID: %w", err)
		}

		session.SystemPrompts = []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, scanner, sessionID, execData.AllowAll)}}
		session.OldHistories = []agentTypes.Message{}
		session.ToolHistories = []agentTypes.Message{}

		userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), trimInput)
		session.Histories = append(session.Histories, agentTypes.Message{
			Role:    "user",
			Content: userText,
		})
		session.UserInput = agentTypes.Message{
			Role:    "user",
			Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
		}
		SaveUserInputHistory(sessionID, userText)

		indexDataBytes, err := json.Marshal(IndexData{SessionID: sessionID})
		if err != nil {
			return nil, fmt.Errorf("json.Marshal: %w", err)
		}

		file, err := os.OpenFile(filesystem.ConfigPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("os.OpenFile: %w", err)
		}

		_, err = file.Write(indexDataBytes)
		if err != nil {
			return nil, fmt.Errorf("file.Write: %w", err)
		}

		err = file.Close()
		if err != nil {
			return nil, fmt.Errorf("file.Close: %w", err)
		}
	}

	session.ID = sessionID

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
