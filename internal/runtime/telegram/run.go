package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session"
	sessionTelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

var (
	voiceMarkerRegex = regexp.MustCompile(`\[SEND_VOICE:([^\]]+)\]`)
	tsPrefixRegex    = regexp.MustCompile(`^ts:\d+\n`)
)

func chatName(in go_bot_telegram.Input) string {
	if in.ChatName != "" {
		return in.ChatName
	}
	return in.Username
}

func inputHasAttachment(in go_bot_telegram.Input) bool {
	if len(in.Photo) > 0 || in.Document != nil {
		return true
	}
	if in.Raw != nil && in.Raw.Message != nil {
		m := in.Raw.Message
		return m.Voice != nil || m.Audio != nil || m.Video != nil || m.VideoNote != nil
	}
	return false
}

func run(ctx context.Context, b *Bot, in go_bot_telegram.Input, attachInputs []go_bot_telegram.Input) error {
	isCallback := in.CallbackData != "" || len(in.CallbackPicks) > 0
	content := strings.TrimSpace(in.Text)
	if content == "" {
		content = strings.TrimSpace(in.Caption)
	}
	hasAttachment := slices.ContainsFunc(attachInputs, inputHasAttachment)
	if !isCallback && content == "" && !hasAttachment {
		return nil
	}
	if content == "/start" || strings.HasPrefix(content, "/start ") || strings.HasPrefix(content, "/start@") {
		return nil
	}

	if isCallback {
		if b.listener != nil && b.listener.OnCallback(ctx, in.ChatID, in.MessageID, in.CallbackData, in.CallbackPicks) {
			return nil
		}
		return nil
	}

	isPrivate := in.Raw == nil || in.Raw.Message == nil || in.Raw.Message.Chat.Type == models.ChatTypePrivate
	_, hasVerifyPending := pending.Get(in.ChatID)
	hasListenerAwait := b.listener != nil && b.listener.IsAwaitingChat(in.ChatID)
	if !isPrivate && !hasVerifyPending && !hasListenerAwait {
		botUsername := strings.TrimSpace(b.client.Status().Username)
		if botUsername == "" {
			return nil
		}
		target := "@" + botUsername
		if !strings.Contains(content, target) {
			return nil
		}
		content = strings.TrimSpace(strings.ReplaceAll(content, target, ""))
		if content == "" && !hasAttachment {
			return nil
		}
	}

	if !utils.IsAuthorized(filesystem.TelegramAuthPath, strconv.FormatInt(in.ChatID, 10)) {
		deleteMsg := func(msgID int, label string) {
			if msgID == 0 {
				return
			}
			if err := b.client.Delete(ctx, in.ChatID, msgID); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Delete",
					slog.String("label", label),
					slog.String("chat", chatName(in)),
					slog.Int("msg", msgID),
					slog.String("error", err.Error()))
			}
		}

		if p, ok := pending.Get(in.ChatID); ok {
			if strings.TrimSpace(in.Text) == p.Code {
				if err := authorizeChat(in); err != nil {
					return fmt.Errorf("authorizeChat: %w", err)
				}
				pending.Clear(in.ChatID)
				deleteMsg(p.PromptMsgID, "prompt")
				deleteMsg(in.MessageID, "code")
				return nil
			}
			deleteMsg(p.PromptMsgID, "prompt")
		}
		deleteMsg(in.MessageID, "unverified")
		code, err := utils.GenerateAuthCode()
		if err != nil {
			return fmt.Errorf("utils.GenerateAuthCode: %w", err)
		}
		slog.Info("Telegram Verification Code",
			slog.String("name", chatName(in)),
			slog.String("code", code))
		exec.NotifyAdminCode(ctx, code, "Telegram "+chatName(in))
		prompt, err := b.client.SendInput(ctx, in.ChatID, 0, "Enter the 6-digit verification code printed in the daemon log.")
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendInput",
				slog.String("chat", chatName(in)),
				slog.String("error", err.Error()))
			return nil
		}
		promptID := 0
		if prompt != nil {
			promptID = prompt.ID
		}
		pending.Set(in.ChatID, code, promptID)
		return nil
	}

	if b.listener != nil && b.listener.OnText(ctx, in.ChatID, in.MessageID, in.Text) {
		return nil
	}

	if hasAttachment {
		var paths []string
		for _, ai := range attachInputs {
			paths = append(paths, saveAttachments(ctx, b, ai)...)
		}
		if len(paths) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, "[Telegram attachments]")
			for _, p := range paths {
				lines = append(lines, "- "+p)
			}
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
		return nil
	}

	markStatus := func(str string) {
		wrapped := fmt.Sprintf("<blockquote expandable>%s</blockquote>", html.EscapeString(str))
		if err := b.client.SendStatus(ctx, in.ChatID, in.MessageID, wrapped, go_bot_telegram.WithStatusSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendStatus",
				slog.String("text", str),
				slog.String("chat", chatName(in)),
				slog.Int("replyTo", in.MessageID),
				slog.String("error", err.Error()))
		}
	}
	markStatus("thinking…")

	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	var sessionOverride, sessionMissing string
	if name, effective := session.CheckAssign(content); name != "" {
		if id := session.GetSessionID(name); id != "" {
			sessionOverride = id
		} else {
			sessionMissing = name
		}
		content = strings.TrimSpace(effective)
	}

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(content)
	if externalAgent != "" {
		content = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *skill.Skill
	if externalAgent == "" && scanner != nil {
		if m, effective := runtime.MatchSkill(scanner, content, tools.TUIOnlySkills...); m != nil {
			matchedSkill = m
			content = strings.TrimSpace(effective)
		}
	}

	routingSessionID := sessionOverride
	if routingSessionID == "" {
		cs, err := sessionTelegram.New(in.ChatID)
		if err != nil {
			return fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetTelegramSession: %w", err)
		}
		routingSessionID = cs
	}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	if externalAgent == "" {
		primary, rest, err := exec.ResolveAgent(ctx, agents.DispatcherBot(), agents.Registry(), content, matchedSkill != nil, routingSessionID)
		if err != nil {
			if finishErr := b.client.FinishStatus(ctx, in.ChatID); finishErr != nil {
				slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.FinishStatus",
					slog.String("chat", chatName(in)),
					slog.String("error", finishErr.Error()))
			}
			errReply := fmt.Sprintf("<blockquote expandable>⚠️ %s</blockquote>", html.EscapeString(err.Error()))
			if _, sendErr := b.client.Send(ctx, in.ChatID, in.MessageID, errReply, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); sendErr != nil {
				slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Send (ResolveAgent error reply)",
					slog.String("chat", chatName(in)),
					slog.String("error", sendErr.Error()))
			}
			return fmt.Errorf("ResolveAgent: %w", err)
		}
		agent = primary
		fallbacks = rest
	}

	execData := exec.ExecData{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Skill:          matchedSkill,
		Content:        content,
		ExcludeTools:   tools.TUIOnlyTools,
		ExcludeSkills:  tools.TUIOnlySkills,
		AllowAll:       false,
	}

	sess, err := getSession(in.ChatID, in.Username, content, execData, sessionOverride, sessionMissing)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}
	utils.EventLog("[Telegram]", agentTypes.Event{}, sess.ID, content)

	events := make(chan agentTypes.Event, 128)
	go func() {
		var execErr error
		execCtx := exec.SuppressDcPush(ctx)
		if externalAgent != "" {
			execErr = exec.CallExternal(execCtx, sess.ID, externalAgent, content, externalReadOnly, events)
		} else {
			execErr = exec.Execute(execCtx, execData, sess, events, execData.AllowAll)
		}
		if execErr != nil {
			slog.Warn("exec",
				slog.String("session", sess.ID),
				slog.String("error", execErr.Error()))
		}
		close(events)
	}()

	result := utils.FormatChatbotEvent(events, "[Telegram]", sess.ID, markStatus, func(toolName, text string) string {
		return fmt.Sprintf("<code>%s</code>: <code>%s</code>", toolName, text)
	})
	replyText := result.ReplyText
	execErrors := result.ExecErrors
	doneEvent := result.Done

	if err := b.client.FinishStatus(ctx, in.ChatID); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.FinishStatus",
			slog.String("session", sess.ID),
			slog.String("chat", chatName(in)),
			slog.String("error", err.Error()))
	}

	replyText = strings.TrimSpace(tsPrefixRegex.ReplaceAllString(replyText, ""))
	replyText = sanitizeHTML(replyText)
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	cleanText, photoPaths, docPaths := extractFileMarkers(replyText)
	replyText = cleanText

	var voiceTexts []string
	for _, match := range voiceMarkerRegex.FindAllStringSubmatch(replyText, -1) {
		if t := strings.TrimSpace(match[1]); t != "" {
			voiceTexts = append(voiceTexts, t)
		}
	}
	replyText = strings.TrimSpace(voiceMarkerRegex.ReplaceAllString(replyText, ""))

	model := doneEvent.Model
	if model == "" && agent != nil {
		model = agent.Name()
	}
	footer := utils.FormatEventFooter(doneEvent.Duration, model, doneEvent.Usage)
	if len(photoPaths) > 0 || len(docPaths) > 0 || len(voiceTexts) > 0 {
		footer = "🔗 " + footer
	}
	replyText = fmt.Sprintf("%s\n\n<blockquote expandable>%s</blockquote>", replyText, footer)
	if len(execErrors) > 0 {
		replyText = fmt.Sprintf("%s\n\n<blockquote expandable>⚠️ %s</blockquote>", replyText, strings.Join(execErrors, ", "))
	}

	if in.MessageID != 0 {
		replyText = "​\n" + replyText
	}
	chunks := chunk(replyText)
	replyTo := in.MessageID
	for _, chunk := range chunks {
		_, sendErr := b.client.Send(ctx, in.ChatID, replyTo, chunk, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
		if sendErr != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Send",
				slog.String("session", sess.ID),
				slog.String("error", sendErr.Error()))
			break
		}
		replyTo = 0
	}

	if len(photoPaths) == 0 && len(docPaths) == 0 && len(voiceTexts) == 0 {
		return nil
	}

	if len(photoPaths) > 0 || len(docPaths) > 0 {
		bgCtx := context.WithoutCancel(ctx)
		chat := chatName(in)
		go sendAttachments(bgCtx, in.ChatID, chat, photoPaths, docPaths)
	}

	if len(voiceTexts) > 0 {
		bgCtx := context.WithoutCancel(ctx)
		chat := chatName(in)
		chatID := in.ChatID
		client := b.client
		texts := voiceTexts
		sessID := sess.ID
		go func() {
			notifyFailure := func(errMsg string) {
				text := fmt.Sprintf("⚠️ SendVoice failed (background)\n<code>%s</code>", html.EscapeString(errMsg))
				if _, err := client.Send(bgCtx, chatID, 0, text, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
					slog.Error("github.com/pardnchiu/go-bot/telegram Bot.client.Send (notify)",
						slog.String("session", sessID),
						slog.String("chat", chat),
						slog.String("error", err.Error()))
				}
			}
			apiKey := strings.TrimSpace(keychain.Get("GEMINI_API_KEY"))
			if apiKey == "" {
				slog.Error("keychain.Get GEMINI_API_KEY missing",
					slog.String("session", sessID),
					slog.String("chat", chat))
				notifyFailure("GEMINI_API_KEY missing")
				return
			}
			for _, text := range texts {
				if _, err := client.SendVoice(bgCtx, chatID, text, apiKey); err != nil {
					slog.Error("github.com/pardnchiu/go-bot/telegram Bot.client.SendVoice",
						slog.String("session", sessID),
						slog.String("chat", chat),
						slog.String("error", err.Error()))
					notifyFailure(err.Error())
				}
			}
		}()
	}

	return nil
}
