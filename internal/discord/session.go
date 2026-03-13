package discord

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const MaxHistoryMessages = 16

func getSession(ctx context.Context, guildID, channelID, userID, input string, imageInputs []string, fileInputs []discordTypes.FileInput, data exec.ExecData) (*agentTypes.AgentSession, error) {
	sessionID, err := filesystem.GetDiscordSessionID(guildID, channelID, userID)
	if err != nil {
		return nil, fmt.Errorf("filesystem.GetDiscordSessionID: %w", err)
	}

	// configDir, err := utils.GetConfigDir("sessions")
	// if err != nil {
	// 	return nil, fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	// sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
	// if err := os.MkdirAll(sessionDir, 0755); err != nil {
	// 	return nil, fmt.Errorf("os.MkdirAll: %w", err)
	// }

	// if configData, err := json.Marshal(map[string]string{
	// 	"guild_id":   guildID,
	// 	"channel_id": channelID,
	// 	"user_id":    userID,
	// }); err == nil {
	// 	configPath := filepath.Join(sessionDir, "config.json")
	// 	if err := filesystem.WriteFile(configPath, string(configData), 0644); err != nil {
	// 		slog.Warn("filesystem.WriteFile config",
	// 			slog.String("error", err.Error()))
	// 	}
	// }

	session := &agentTypes.AgentSession{
		ID:    sessionID,
		Tools: []agentTypes.Message{},
		Messages: []agentTypes.Message{
			{Role: "system", Content: exec.GetSystemPrompt(data)},
			{Role: "system", Content: configs.DiscordSystemPrompt},
		},
		Histories: []agentTypes.Message{},
	}

	oldHistory := exec.BytesToHistory(sessionID)
	session.Histories = oldHistory
	session.Messages = append(session.Messages, oldHistory...)
	// historyPath := filepath.Join(sessionDir, "history.json")
	// if historyData, err := os.ReadFile(historyPath); err == nil {
	// 	var oldHistory []agentTypes.Message
	// 	if json.Unmarshal(historyData, &oldHistory) == nil {
	// 		session.Histories = oldHistory
	// 	}
	// 	if len(oldHistory) > MaxHistoryMessages {
	// 		oldHistory = oldHistory[len(oldHistory)-MaxHistoryMessages:]
	// 	}
	// 	session.Messages = append(session.Messages, oldHistory...)
	// }
	//
	if summary := filesystem.GetSummary(sessionID); summary != "" {
		session.Messages = append(session.Messages, agentTypes.Message{
			Role:    "system",
			Content: summary,
		})
	}
	// summaryPath := filepath.Join(sessionDir, "summary.json")
	// if summaryData, err := os.ReadFile(summaryPath); err == nil {
	// 	summary := strings.NewReplacer(
	// 		"{{.Summary}}", string(summaryData),
	// 	).Replace(strings.TrimSpace(configs.SummaryPrompt))
	// 	session.Messages = append(session.Messages, agentTypes.Message{
	// 		Role:    "system",
	// 		Content: summary,
	// 	})
	// }

	userText := fmt.Sprintf("當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), strings.TrimSpace(input))

	var userContent any
	if len(imageInputs) > 0 || len(fileInputs) > 0 {
		parts := []agentTypes.ContentPart{
			{Type: "text", Text: userText},
		}

		for _, imageInput := range imageInputs {
			dataURL, err := fetchImageDataURL(ctx, imageInput)
			if err != nil {
				slog.Warn("fetchImageDataURL",
					slog.String("error", err.Error()))
				dataURL = imageInput
			}
			parts = append(parts, agentTypes.ContentPart{
				Type:     "image_url",
				ImageURL: &agentTypes.ImageURL{URL: dataURL},
			})
		}

		for _, fileInput := range fileInputs {
			text, err := fetchFileText(ctx, fileInput.URL)
			if err != nil {
				slog.Warn("fetchFileText",
					slog.String("error", err.Error()))
				continue
			}
			parts = append(parts, agentTypes.ContentPart{
				Type: "text",
				Text: fmt.Sprintf("----------\n%s\n----------\n%s", fileInput.Name, text),
			})
		}
		userContent = parts
	} else {
		userContent = userText
	}

	session.Histories = append(session.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	session.Messages = append(session.Messages, agentTypes.Message{
		Role:    "user",
		Content: userContent,
	})

	return session, nil
}

func getSessionID(guildID, channelID, userID string) string {
	if guildID == "" {
		guildID = "dm"
	}
	if channelID == "" {
		channelID = "ch"
	}
	key := fmt.Sprintf("%s_%s_%s", guildID, channelID, userID)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func fetchImageDataURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http.DefaultClient.Do: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		base := strings.SplitN(rawURL, "?", 2)[0]
		ext := strings.ToLower(filepath.Ext(base))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		default:
			contentType = "image/jpeg"
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)), nil
}

func fetchFileText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http.DefaultClient.Do: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	return string(data), nil
}
