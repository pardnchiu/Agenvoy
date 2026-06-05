package chatbot

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/stt"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type Channel int

const (
	Telegram Channel = iota
	Discord
)

var VoiceMarkerRegex = regexp.MustCompile(`\[SEND_VOICE:([^\]]+)\]`)

type SavedAttachment struct {
	Path       string
	Transcribe bool
}

func RuntimeExcludeTools(autoTranscribed bool) []string {
	excluded := append([]string{}, tools.TUIOnlyTools...)
	if autoTranscribed {
		excluded = append(excluded, "transcribe_media")
	}
	return excluded
}

func TranscribeSavedAttachments(ctx context.Context, attachments []SavedAttachment) ([]string, []string, error) {
	var transcripts []string
	var paths []string
	for _, attachment := range attachments {
		if attachment.Path == "" {
			continue
		}
		if !attachment.Transcribe {
			paths = append(paths, attachment.Path)
			continue
		}
		text, err := stt.Transcribe(ctx, attachment.Path, "")
		if err != nil {
			return nil, nil, fmt.Errorf("transcribe %s: %w", attachment.Path, err)
		}
		if text = strings.TrimSpace(text); text != "" {
			transcripts = append(transcripts, text)
		}
	}
	return transcripts, paths, nil
}

type VoiceExtractResult struct {
	CleanText string
	Texts     []string
	AutoReply bool
}

func SendAdminCode(ctx context.Context, ch Channel, targetID, text string) error {
	switch ch {
	case Telegram:
		token := strings.TrimSpace(keychain.Get("TELEGRAM_TOKEN"))
		if token == "" {
			return fmt.Errorf("telegram token missing")
		}
		id, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil {
			return fmt.Errorf("parse chatID %q: %w", targetID, err)
		}
		client, err := go_bot_telegram.New(token)
		if err != nil {
			return fmt.Errorf("go-bot/telegram New: %w", err)
		}
		if _, err := client.Send(ctx, id, 0, html.EscapeString(text), go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			return fmt.Errorf("go-bot/telegram Send: %w", err)
		}
	case Discord:
		token := strings.TrimSpace(keychain.Get("DISCORD_TOKEN"))
		if token == "" {
			return fmt.Errorf("discord token missing")
		}
		client, err := go_bot_discord.New(token)
		if err != nil {
			return fmt.Errorf("go-bot/discord New: %w", err)
		}
		if _, err := client.Send(ctx, strings.TrimSpace(targetID), "", text); err != nil {
			return fmt.Errorf("go-bot/discord Send: %w", err)
		}
	default:
		return fmt.Errorf("unknown channel %d", ch)
	}
	return nil
}

func wrapBlock(ch Channel, text string) string {
	switch ch {
	case Telegram:
		return "<blockquote expandable>" + text + "</blockquote>"
	default:
		return "-# ⎿ " + text
	}
}

func BuildPushFooter(ch Channel, duration time.Duration, model string, usage *agentTypes.Usage) string {
	footer := utils.FormatEventFooter(duration, model, usage)
	if footer == "" {
		return ""
	}
	switch ch {
	case Telegram:
		return "\n\n" + wrapBlock(ch, footer)
	default:
		return "\n" + wrapBlock(ch, footer)
	}
}

func AppendReplyFooter(ch Channel, text, footer string, hasMedia bool, execErrors []string) string {
	if hasMedia {
		footer = "🔗 " + footer
	}
	switch ch {
	case Telegram:
		text = fmt.Sprintf("%s\n\n%s", text, wrapBlock(ch, footer))
	default:
		text = fmt.Sprintf("%s\n%s", text, wrapBlock(ch, footer))
	}
	if len(execErrors) > 0 {
		errLine := wrapBlock(ch, "⚠️ "+strings.Join(execErrors, ", "))
		switch ch {
		case Telegram:
			text = fmt.Sprintf("%s\n\n%s", text, errLine)
		default:
			text = fmt.Sprintf("%s\n%s", text, errLine)
		}
	}
	return text
}

func ExtractVoiceMarkers(replyText string, autoTranscribed bool) VoiceExtractResult {
	var texts []string
	for _, match := range VoiceMarkerRegex.FindAllStringSubmatch(replyText, -1) {
		if t := strings.TrimSpace(match[1]); t != "" {
			texts = append(texts, t)
		}
	}
	clean := strings.TrimSpace(VoiceMarkerRegex.ReplaceAllString(replyText, ""))
	autoReply := false
	if autoTranscribed && len(texts) == 0 {
		if t := utils.CleanVoiceReplyText(clean); t != "" {
			texts = append(texts, t)
			autoReply = true
		}
	}
	return VoiceExtractResult{
		CleanText: clean,
		Texts:     texts,
		AutoReply: autoReply,
	}
}
