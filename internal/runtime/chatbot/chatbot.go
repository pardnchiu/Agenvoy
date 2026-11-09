package chatbot

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode"

	go_bot_discord "github.com/pardnchiu/go-bot/core/discord"
	go_bot_line "github.com/pardnchiu/go-bot/core/line"
	go_bot_telegram "github.com/pardnchiu/go-bot/core/telegram"
	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Channel int

const (
	Telegram Channel = iota
	Discord
	Line
)

type SavedAttachment struct {
	Path       string
	Transcribe bool
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
		text, err := filesystem.TranscribeMedia(ctx, attachment.Path)
		if err != nil {
			return nil, nil, utilsmsg.Trace("transcribe", attachment.Path, err)
		}
		if text = strings.TrimSpace(text); text != "" {
			transcripts = append(transcripts, text)
		}
	}
	return transcripts, paths, nil
}

func SendAdminCode(ctx context.Context, ch Channel, targetID, text string) error {
	switch ch {
	case Telegram:
		token := strings.TrimSpace(keychain.Get("TELEGRAM_TOKEN"))
		if token == "" {
			return utilsmsg.New(utilsmsg.Denied, "telegram token", "missing")
		}
		id, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil {
			return utilsmsg.New(utilsmsg.Invalid, "chatID", "parse %q: %w", targetID, err)
		}
		client, err := go_bot_telegram.New(token)
		if err != nil {
			return utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.New", "", err)
		}
		if _, err := client.Send(ctx, id, 0, html.EscapeString(text), go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			return utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.Send", "", err)
		}
	case Discord:
		token := strings.TrimSpace(keychain.Get("DISCORD_TOKEN"))
		if token == "" {
			return utilsmsg.New(utilsmsg.Denied, "discord token", "missing")
		}
		client, err := go_bot_discord.New(token)
		if err != nil {
			return utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_discord.New", "", err)
		}
		if _, err := client.Send(ctx, strings.TrimSpace(targetID), "", text); err != nil {
			return utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_discord.Send", "", err)
		}
	case Line:
		secret := strings.TrimSpace(keychain.Get("LINE_SECRET"))
		token := strings.TrimSpace(keychain.Get("LINE_TOKEN"))
		if secret == "" || token == "" {
			return fmt.Errorf("line secret or token missing")
		}
		client, err := go_bot_line.New(secret, token, filesystem.LinePort)
		if err != nil {
			return fmt.Errorf("go-bot/line New: %w", err)
		}
		if _, err := client.Send(ctx, strings.TrimSpace(targetID), text); err != nil {
			return fmt.Errorf("go-bot/line Send: %w", err)
		}
	default:
		return utilsmsg.New(utilsmsg.Internal, "channel", "unknown %d", ch)
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

func BuildPushFooter(ch Channel, duration, outputElapsed time.Duration, model, reasoning string, usage *go_llm_router.Usage) string {
	footer := utils.FormatEventFooter(duration, outputElapsed, model, reasoning, usage)
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

func CutNewCommand(content string) (string, bool) {
	head, rest := content, ""
	if i := strings.IndexFunc(content, unicode.IsSpace); i >= 0 {
		head, rest = content[:i], content[i:]
	}
	if cmd, _, _ := strings.Cut(head, "@"); cmd != "/new" {
		return content, false
	}
	return strings.TrimSpace(rest), true
}
