package discord

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
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/bwmarrin/discordgo"
	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func getSession(ctx context.Context, dcSession *discordgo.Session, guildID, channelID, userID, currentMessageID, input string, imageInputs []string, fileInputs []discordTypes.FileInput, data exec.ExecData) (*agentTypes.AgentSession, error) {
	sessionID, err := sessionManager.GetDiscordSession(guildID, channelID, userID)
	if err != nil {
		return nil, fmt.Errorf("sessionManager.GetDiscordSessionID: %w", err)
	}

	session := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	var channelHistory []agentTypes.Message
	if msgs, err := dcSession.ChannelMessages(channelID, 16, currentMessageID, "", ""); err == nil {
		botID := dcSession.State.User.ID
		for i := len(msgs) - 1; i >= 0; i-- {
			msg := msgs[i]
			if msg.Author == nil || msg.Content == "" {
				continue
			}
			role := "user"
			content := msg.Content
			if msg.Author.ID == botID {
				role = "assistant"
				if idx := strings.LastIndex(content, "\n-# "); idx != -1 {
					content = content[:idx]
				}
			}
			channelHistory = append(channelHistory, agentTypes.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	session.SystemPrompts = []agentTypes.Message{
		{Role: "system", Content: configs.DiscordSystemPrompt},
		{Role: "system", Content: exec.GetSystemPrompt(data)},
	}
	if summary := sessionManager.GetSummaryPrompt(sessionID, time.Time{}); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	session.OldHistories = channelHistory
	session.ToolHistories = []agentTypes.Message{}

	userText := fmt.Sprintf("當前時間: %s\n當前頻道 ID: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), channelID, strings.TrimSpace(input))

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
				ImageURL: &agentTypes.ImageURL{URL: dataURL, Detail: "auto"},
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

	session.UserInput = agentTypes.Message{
		Role:    "user",
		Content: userContent,
	}

	return session, nil
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

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", fmt.Errorf("image.Decode: %w", err)
	}

	// * need to be use jpeg before send in claude/gemini model
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("jpeg.Encode: %w", err)
	}

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
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
