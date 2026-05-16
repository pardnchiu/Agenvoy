package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

var (
	fileMarkerRegex  = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)
	voiceMarkerRegex = regexp.MustCompile(`\[SEND_VOICE:([^\]]+)\]`)
	tsPrefixRegex    = regexp.MustCompile(`^ts:\d+\n`)
	imageExts        = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
)

func truncateStatus(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return string(r)
}

func fmtUsage(n int) string {
	if n > 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

func run(ctx context.Context, b *Bot, in go_bot_telegram.Input) error {
	isCallback := in.CallbackData != "" || len(in.CallbackPicks) > 0
	content := strings.TrimSpace(in.Text)
	if content == "" {
		content = strings.TrimSpace(in.Caption)
	}
	if !isCallback && content == "" {
		return nil
	}
	if content == "/start" || strings.HasPrefix(content, "/start ") || strings.HasPrefix(content, "/start@") {
		return nil
	}

	if isCallback {
		if b.listener != nil && b.listener.onCallback(ctx, in.ChatID, in.MessageID, in.CallbackData, in.CallbackPicks) {
			return nil
		}
		return nil
	}

	if !isAuthorized(in.ChatID) {
		deleteMsg := func(msgID int, label string) {
			if msgID == 0 {
				return
			}
			if err := b.client.Delete(ctx, in.ChatID, msgID); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Delete",
					slog.String("label", label),
					slog.Int64("chat", in.ChatID),
					slog.Int("msg", msgID),
					slog.String("error", err.Error()))
			}
		}

		if p, pending := getPending(in.ChatID); pending {
			if strings.TrimSpace(in.Text) == p.code {
				if err := authorizeChat(in.ChatID); err != nil {
					return fmt.Errorf("authorizeChat: %w", err)
				}
				clearPending(in.ChatID)
				deleteMsg(p.promptMsgID, "prompt")
				deleteMsg(in.MessageID, "code")
				return nil
			}
			deleteMsg(p.promptMsgID, "prompt")
		}
		deleteMsg(in.MessageID, "unverified")
		code, err := generateCode()
		if err != nil {
			return fmt.Errorf("generateCode: %w", err)
		}
		slog.Info("Telegram Verification Code",
			slog.Int64("chat", in.ChatID),
			slog.String("username", in.Username),
			slog.String("code", code))
		prompt, err := b.client.SendInput(ctx, in.ChatID, 0, "Enter the 6-digit verification code printed in the daemon log.")
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendInput",
				slog.Int64("chat", in.ChatID),
				slog.String("error", err.Error()))
			return nil
		}
		promptID := 0
		if prompt != nil {
			promptID = prompt.ID
		}
		setPending(in.ChatID, code, promptID)
		return nil
	}

	if b.listener != nil && b.listener.onText(ctx, in.ChatID, in.MessageID, in.Text) {
		return nil
	}

	markStatus := func(text string) {
		wrapped := fmt.Sprintf("<blockquote expandable>%s</blockquote>", html.EscapeString(text))
		if err := b.client.SendStatus(ctx, in.ChatID, in.MessageID, wrapped, go_bot_telegram.WithStatusSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendStatus",
				slog.String("text", text),
				slog.Int64("chat", in.ChatID),
				slog.Int("replyTo", in.MessageID),
				slog.String("error", err.Error()))
		}
	}
	markStatus("thinking…")

	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	scanner := host.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	var sessionOverride, sessionMissing string
	if name, effective := session.Match(content); name != "" {
		if id := session.GetSessionIDByName(name); id != "" {
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
		if m, effective := scanner.MatchSkillCall(content); m != nil {
			matchedSkill = m
			content = strings.TrimSpace(effective)
			slog.Info("skill", slog.String("skill", m.Name))
		}
	}

	var agent agentTypes.Agent
	if externalAgent == "" {
		agent = exec.SelectAgent(ctx, host.Planner(), host.Registry(), content, matchedSkill != nil, "")
	}

	execData := exec.ExecData{
		Agent:    agent,
		WorkDir:  workDir,
		Skill:    matchedSkill,
		Content:  content,
		AllowAll: false,
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
				slog.String("error", execErr.Error()))
		}
		close(events)
	}()

	var replyText string
	var execErrors []string
	var doneEvent agentTypes.Event
	for e := range events {
		utils.EventLog("[Telegram]", e, sess.ID, "")
		switch e.Type {
		case agentTypes.EventAgentSelect:
			markStatus("selecting agent…")

		case agentTypes.EventAgentResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				markStatus("(agent) " + truncateStatus(t))
			}

		case agentTypes.EventSkillResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				markStatus("(skill)  " + truncateStatus(t))
			}

		case agentTypes.EventToolCall:
			if e.ToolName != "" {
				markStatus("(tool) " + e.ToolName)
			}

		case agentTypes.EventToolSkipped:
			if e.ToolName != "" {
				markStatus("(tool skipped) " + e.ToolName)
			}

		case agentTypes.EventSummaryGenerate:
			markStatus("summarizing…")

		case agentTypes.EventText:
			if replyText != "" {
				replyText += "\n"
			}
			replyText += e.Text

		case agentTypes.EventExecError:
			execErrors = append(execErrors, fmt.Sprintf("<code>%s</code>: <code>%s</code>", e.ToolName, e.Text))

		case agentTypes.EventDone:
			doneEvent = e
		}
	}

	if err := b.client.FinishStatus(ctx, in.ChatID); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.FinishStatus",
			slog.Int64("chat", in.ChatID),
			slog.String("error", err.Error()))
	}

	replyText = strings.TrimSpace(tsPrefixRegex.ReplaceAllString(replyText, ""))
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	var filePaths []string
	for _, match := range fileMarkerRegex.FindAllStringSubmatch(replyText, -1) {
		filePaths = append(filePaths, strings.TrimSpace(match[1]))
	}
	replyText = strings.TrimSpace(fileMarkerRegex.ReplaceAllString(replyText, ""))

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
	if _, after, ok := strings.Cut(model, "@"); ok {
		model = after
	}
	footer := model
	if doneEvent.Usage != nil {
		footer = fmt.Sprintf("%s | in:%s out:%s", footer, fmtUsage(doneEvent.Usage.Input), fmtUsage(doneEvent.Usage.Output))
	}
	replyText = fmt.Sprintf("%s\n\n<blockquote expandable>%s</blockquote>", replyText, footer)
	if len(execErrors) > 0 {
		replyText = fmt.Sprintf("%s\n\n<blockquote expandable>⚠️ %s</blockquote>", replyText, strings.Join(execErrors, ", "))
	}

	if in.MessageID != 0 {
		replyText = "​\n" + replyText
	}
	replyMsg, sendErr := b.client.Send(ctx, in.ChatID, in.MessageID, replyText, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
	if sendErr != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Send",
			slog.String("error", sendErr.Error()))
	}

	var photoPaths []string
	var docPaths []string
	for _, path := range filePaths {
		if imageExts[strings.ToLower(filepath.Ext(path))] {
			photoPaths = append(photoPaths, path)
			continue
		}
		docPaths = append(docPaths, path)
	}

	if len(photoPaths) == 0 && len(docPaths) == 0 && len(voiceTexts) == 0 {
		return nil
	}

	replyToID := 0
	if replyMsg != nil {
		replyToID = replyMsg.ID
	}
	sendStatus := func(text string) {
		wrapped := fmt.Sprintf("<blockquote expandable>%s</blockquote>", html.EscapeString(text))
		if err := b.client.SendStatus(ctx, in.ChatID, replyToID, wrapped,
			go_bot_telegram.WithStatusEmoji("⚡"),
			go_bot_telegram.WithStatusSendType(go_bot_telegram.TypeHTML),
		); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendStatus",
				slog.String("text", text),
				slog.Int64("chat", in.ChatID),
				slog.Int("replyTo", replyToID),
				slog.String("error", err.Error()))
		}
	}
	sendFailure := func(label, detail, errMsg string) {
		body := fmt.Sprintf("<code>%s</code>", html.EscapeString(errMsg))
		if detail != "" {
			body = fmt.Sprintf("<code>%s</code>: %s", html.EscapeString(detail), body)
		}
		text := fmt.Sprintf("⚠️ %s failed\n%s", label, body)
		if _, err := b.client.Send(ctx, in.ChatID, replyToID, text, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}
	sendStatus("sending…")

	for start := 0; start < len(photoPaths); start += 10 {
		end := start + 10
		end = min(end, len(photoPaths))
		if _, err := b.client.SendPhoto(ctx, in.ChatID, photoPaths[start:end]); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendPhoto",
				slog.Int("count", end-start),
				slog.String("error", err.Error()))
			sendFailure("SendPhoto", strings.Join(photoPaths[start:end], ", "), err.Error())
		}
	}
	for _, path := range docPaths {
		if _, err := b.client.SendFile(ctx, in.ChatID, go_bot_telegram.TypeDocument, path); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendFile",
				slog.String("path", path),
				slog.String("error", err.Error()))
			sendFailure("SendFile", path, err.Error())
		}
	}

	if len(voiceTexts) > 0 {
		apiKey := strings.TrimSpace(keychain.Get("GEMINI_API_KEY"))
		if apiKey == "" {
			slog.Warn("keychain.Get GEMINI_API_KEY missing",
				slog.Int64("chat", in.ChatID))
			sendFailure("SendVoice", "", "GEMINI_API_KEY missing")
		} else {
			for _, text := range voiceTexts {
				if _, err := b.client.SendVoice(ctx, in.ChatID, text, apiKey); err != nil {
					slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.SendVoice",
						slog.String("error", err.Error()))
					sendFailure("SendVoice", "", err.Error())
				}
			}
		}
	}

	if err := b.client.FinishStatus(ctx, in.ChatID); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.client.FinishStatus",
			slog.Int64("chat", in.ChatID),
			slog.String("error", err.Error()))
	}

	return nil
}
