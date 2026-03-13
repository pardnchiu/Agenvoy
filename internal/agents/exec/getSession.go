package exec

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/sessionManager"
)

const (
	MaxHistoryMessages = 16
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
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parts = append(parts, agentTypes.ContentPart{
			Type: "text",
			Text: fmt.Sprintf("---\npath: %s\n---\n%s", filepath.Base(path), string(data)),
		})
	}

	for _, path := range imageInputs {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		mime := http.DetectContentType(data)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
		parts = append(parts, agentTypes.ContentPart{
			Type:     "image_url",
			ImageURL: &agentTypes.ImageURL{URL: dataURL},
		})
	}
	return parts
}

func GetSession(execData ExecData) (*agentTypes.AgentSession, error) {
	prompt := GetSystemPrompt(execData)
	trimInput := strings.TrimSpace(execData.Content)
	session := agentTypes.AgentSession{
		Tools: []agentTypes.Message{},
		Messages: []agentTypes.Message{
			{
				Role:    "system",
				Content: prompt,
			},
		},
		Histories: []agentTypes.Message{},
	}

	unlock, err := sessionManager.LockConfig()
	if err != nil {
		return nil, fmt.Errorf("lockConfig: %w", err)
	}
	defer unlock()

	var sessionID string
	data, configErr := os.ReadFile(filesystem.ConfigPath)
	switch {
	case configErr == nil:
		// * config is exist
		var indexData IndexData
		if err := json.Unmarshal(data, &indexData); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}
		if indexData.SessionID == "" {
			newID, err := sessionManager.CreateSession()
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
			if err := filesystem.WriteFile(filesystem.ConfigPath, string(merged), 0644); err != nil {
				return nil, fmt.Errorf("utils.WriteFile: %w", err)
			}
			indexData.SessionID = newID
		}
		sessionID = strings.TrimSpace(indexData.SessionID)

		oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
		session.Histories = oldHistory
		session.Messages = append(session.Messages, maxHistory...)

		// * insert summary prompt every time
		if summary := sessionManager.GetSummaryPrompt(sessionID); summary != "" {
			session.Messages = append(session.Messages, agentTypes.Message{
				Role:    "system",
				Content: summary,
			})
		}

		userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), trimInput)
		session.Histories = append(session.Histories, agentTypes.Message{
			Role:    "user",
			Content: userText,
		})
		session.Messages = append(session.Messages, agentTypes.Message{
			Role:    "user",
			Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
		})

	case os.IsNotExist(configErr):
		// * config is not exist
		sessionID, err := sessionManager.CreateSession()
		if err != nil {
			return nil, fmt.Errorf("newSessionID: %w", err)
		}

		userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), trimInput)
		session.Histories = append(session.Histories, agentTypes.Message{
			Role:    "user",
			Content: userText,
		})
		session.Messages = append(session.Messages, agentTypes.Message{
			Role:    "user",
			Content: buildContent(userText, execData.ImageInputs, execData.FileInputs),
		})

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

	default:
		return nil, fmt.Errorf("os.ReadFile: %w", configErr)
	}

	session.ID = sessionID

	return &session, nil
}
