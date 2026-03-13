package exec

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
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
		{Type: "text", Text: content},
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

	now := fmt.Sprintf("%d", time.Now().Unix())
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

	// configDir, err := utils.GetConfigDir("sessions")
	// if err != nil {
	// 	return nil, fmt.Errorf("utils.ConfigDir: %v\n", err)
	// }

	// indexJsonPath := filepath.Join(filesystem.AgenvoyDir, "config.json")
	unlock, err := lockConfig(filesystem.AgenvoyDir)
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
			newID, err := newSessionID()
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

		// var summary string
		// if summaryData, err := os.ReadFile(filepath.Join(filesystem.SessionsDir, sessionID, "summary.json")); err == nil {
		// 	summary = strings.NewReplacer(
		// 		"{{.Summary}}", string(summaryData),
		// 	).Replace(strings.TrimSpace(configs.SummaryPrompt))
		// }

		oldHistory := BytesToHistory(sessionID)
		session.Histories = oldHistory
		session.Messages = append(session.Messages, oldHistory...)

		// * insert summary prompt every time
		if summary := filesystem.GetSummary(sessionID); summary != "" {
			session.Messages = append(session.Messages, agentTypes.Message{
				Role:    "system",
				Content: summary,
			})
		}

		// if historyData, err := os.ReadFile(filepath.Join(filesystem.SessionsDir, sessionID, "history.json")); err == nil {
		// 	// * for ensuring context relevance
		// 	var oldHistory []agentTypes.Message
		// 	if err := json.Unmarshal(historyData, &oldHistory); err == nil {
		// 		session.Histories = oldHistory
		// 	}
		// 	if len(oldHistory) > MaxHistoryMessages {
		// 		oldHistory = oldHistory[len(oldHistory)-MaxHistoryMessages:]
		// 	}
		// 	session.Messages = append(session.Messages, oldHistory...)

		// 	// * insert summary prompt every time
		// 	if summary != "" {
		// 		session.Messages = append(session.Messages, agentTypes.Message{
		// 			Role:    "system",
		// 			Content: summary,
		// 		})
		// 	}
		// }

		userText := fmt.Sprintf("ts:%s\n%s", now, trimInput)
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
		sessionID, err := newSessionID()
		if err != nil {
			return nil, fmt.Errorf("newSessionID: %w", err)
		}

		userText := fmt.Sprintf("ts:%s\n%s", now, trimInput)
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

	err = os.MkdirAll(filepath.Join(filesystem.SessionsDir, sessionID), 0755)
	if err != nil {
		return nil, fmt.Errorf("os.MkdirAll: %w", err)
	}

	session.ID = sessionID

	return &session, nil
}

func lockConfig(dir string) (func(), error) {
	lockPath := filepath.Join(dir, "config.json.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("os.OpenFile: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("syscall.Flock: %w", err)
	}

	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

func BytesToHistory(sessionID string) []agentTypes.Message {
	if historyBytes := filesystem.GetHistory(sessionID); historyBytes != nil {
		var oldHistory []agentTypes.Message
		if json.Unmarshal(historyBytes, &oldHistory) == nil {
			if len(oldHistory) > MaxHistoryMessages {
				oldHistory = oldHistory[len(oldHistory)-MaxHistoryMessages:]
			}
			return oldHistory
		}
	}
	return nil
}
