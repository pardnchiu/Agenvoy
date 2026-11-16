package chatbot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Reply struct {
	Channel   Channel
	SessionID string
	ReplyTo   string
	Clean     func(text string) string
	Status    func(text string)
	Finish    func()
	Send      func(replyTo, text string) (string, error)
	Attach    func(replyTo string, paths []string)
}

func (r Reply) Run(ctx context.Context, data exec.ExecuteMeta) error {
	if data.WorkDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			r.Finish()
			return utilsmsg.Wrap(utilsmsg.Internal, "os.UserHomeDir", "", err)
		}
		data.WorkDir = home
	}

	origin := "dc-"
	if r.Channel == Telegram {
		origin = "tg-"
	}
	data = exec.Prepare(data)
	execCtx := agentstypes.WithOrigin(exec.SuppressDcPush(ctx), origin)
	events, wait := exec.Stream(execCtx, data.SessionID, 128, func(stream chan<- agentstypes.Event) error {
		return exec.Run(execCtx, data, stream, false)
	})
	return r.deliver(events, wait)
}

func (r Reply) deliver(events <-chan agentstypes.Event, wait func() error) error {
	tag := "[Discord]"
	toolError := func(toolName, text string) string {
		return fmt.Sprintf("`%s`: %s", toolName, text)
	}
	notice := func(text string) string {
		return text
	}
	if r.Channel == Telegram {
		tag = "[Telegram]"
		toolError = func(toolName, text string) string {
			return fmt.Sprintf("<code>%s</code>: <code>%s</code>", toolName, text)
		}
		notice = func(text string) string {
			return "<blockquote expandable>" + html.EscapeString(text) + "</blockquote>"
		}
	}

	result := utils.FormatChatbotEvent(events, tag, r.SessionID, r.Status, toolError)
	execErr := wait()
	r.Finish()
	if execErr != nil {
		slog.Debug("exec",
			slog.String("session", r.SessionID),
			slog.String("error", execErr.Error()))
	}

	text := result.ReplyText
	if r.Clean != nil {
		text = r.Clean(text)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if execErr == nil {
			return utilsmsg.New(utilsmsg.Upstream, "reply", "empty")
		}
		if _, err := r.Send(r.ReplyTo, notice("⚠️ "+execErr.Error())); err != nil {
			slog.Warn(tag+" send exec error",
				slog.String("session", r.SessionID),
				slog.String("error", err.Error()))
		}
		return utilsmsg.Trace("exec", "", execErr)
	}

	text, paths := utils.ExtractFileMarkers(text)
	footer := utils.FormatEventFooter(result.Done.Duration, result.Done.OutputElapsed, result.Done.Usage, time.Now().Format("15:04"))
	text = AppendReplyFooter(r.Channel, text, footer, len(paths) > 0, result.ExecErrors)
	if r.Channel == Telegram {
		text = SanitizeTelegramHTML(text)
	}

	replyTo := r.ReplyTo
	lastID := ""
	for _, part := range Chunk(r.Channel, text) {
		id, err := r.Send(replyTo, part)
		if err != nil {
			slog.Error(tag+" send",
				slog.String("session", r.SessionID),
				slog.String("error", err.Error()))
			if _, noticeErr := r.Send(r.ReplyTo, notice("⚠️ send failed: "+err.Error())); noticeErr != nil {
				slog.Warn(tag+" send failure notice",
					slog.String("session", r.SessionID),
					slog.String("error", noticeErr.Error()))
			}
			break
		}
		lastID = id
		replyTo = ""
	}

	if len(paths) > 0 {
		go r.Attach(lastID, paths)
	}
	return nil
}
