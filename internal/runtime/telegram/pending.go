package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
)

const tgPrefix = "tg-"

type questionType int

const (
	typeToolConfirm questionType = iota
	typeAskText
	typeAskSecret
	typeAskSingle
	typeAskMulti
)

type active struct {
	pendingID string
	chatID    int64
	message   int
	kind      questionType
	questions []runtime.Question
	selected  int
	answers   []any
}

type pendingListener struct {
	bot      *Bot
	cancelFn context.CancelFunc
	mu       sync.Mutex
	cur      *active
	wakeup   chan struct{}
}

func newPendingListener(bot *Bot) *pendingListener {
	ctx, cancel := context.WithCancel(context.Background())
	listener := &pendingListener{
		bot:      bot,
		cancelFn: cancel,
		wakeup:   make(chan struct{}, 1),
	}
	go listener.run(ctx)
	return listener
}

func (l *pendingListener) stop() {
	if l == nil {
		return
	}
	l.cancelFn()
}

func (l *pendingListener) run(ctx context.Context) {
	unregister := runtime.RegisterListener(tgPrefix)
	defer unregister()

	for {
		l.emit(ctx)
		select {
		case <-ctx.Done():
			return
		case <-runtime.Notify:
		case <-l.wakeup:
		}
	}
}

func (l *pendingListener) emit(ctx context.Context) {
	for {
		l.mu.Lock()
		busy := l.cur != nil
		l.mu.Unlock()
		if busy {
			return
		}

		id, next, ok := runtime.PickNext(tgPrefix)
		if !ok {
			return
		}
		if next.Ctx != nil {
			if err := next.Ctx.Err(); err != nil {
				runtime.Resolve(id, runtime.Reply{Error: err})
				continue
			}
		}
		l.sendPrompt(ctx, id, next)
	}
}

func (l *pendingListener) sendPrompt(ctx context.Context, id string, req runtime.Request) {
	chatStr, err := sessionManager.GetChatID(req.SessionID)
	if err != nil {
		slog.Warn("pendingListener.GetChatID",
			slog.String("session", req.SessionID),
			slog.String("error", err.Error()))
		runtime.Resolve(id, runtime.Reply{Error: fmt.Errorf("GetChatID: %w", err)})
		return
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil {
		runtime.Resolve(id, runtime.Reply{Error: fmt.Errorf("parse chatID %q: %w", chatStr, err)})
		return
	}

	switch req.Kind {
	case runtime.KindToolConfirm:
		l.startConfirm(ctx, id, chatID, req)
	case runtime.KindAskUser:
		l.startAskUser(ctx, id, chatID, req)
	default:
		runtime.Resolve(id, runtime.Reply{Error: fmt.Errorf("unknown pending kind: %s", req.Kind)})
	}
}

func (l *pendingListener) startConfirm(ctx context.Context, id string, chatID int64, req runtime.Request) {
	const limit = 3200

	var args, htmlBody string
	if req.ToolName == "run_command" {
		var parsed struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(req.ToolArgs), &parsed); err == nil && len(parsed.Argv) > 0 {
			args = strings.Join(parsed.Argv, " ")
			htmlBody = fmt.Sprintf("<code>%s</code>", html.EscapeString(args))
		}
	}
	if args == "" {
		args = strings.ReplaceAll(req.ToolArgs, "\r", "")
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(args), "", "  "); err == nil {
			args = pretty.String()
		}
		htmlBody = fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(args))
	}

	var text string
	var modes []go_bot_telegram.MessageOption
	if r := []rune(args); len(r) > limit {
		text = fmt.Sprintf("Run %s?\n\n%s...", req.ToolName, string(r[:limit]))
	} else {
		text = fmt.Sprintf("Run %s?\n\n%s", html.EscapeString(req.ToolName), htmlBody)
		modes = append(modes, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
	}

	msg, err := l.bot.client.SendSelect(ctx, chatID, 0, text, confirmOptions, modes...)
	if err != nil {
		runtime.Resolve(id, runtime.Reply{Error: fmt.Errorf("SendSelect: %w", err)})
		return
	}
	l.mu.Lock()
	l.cur = &active{
		pendingID: id,
		chatID:    chatID,
		message:   msg.ID,
		kind:      typeToolConfirm,
	}
	l.mu.Unlock()
}

func (l *pendingListener) startAskUser(ctx context.Context, id string, chatID int64, req runtime.Request) {
	if req.AskUser == nil || len(req.AskUser.Questions) == 0 {
		runtime.Resolve(id, runtime.Reply{Error: fmt.Errorf("ask_user with no questions")})
		return
	}
	state := &active{
		pendingID: id,
		chatID:    chatID,
		questions: req.AskUser.Questions,
		answers:   make([]any, 0, len(req.AskUser.Questions)),
	}
	if err := l.askNext(ctx, state); err != nil {
		runtime.Resolve(id, runtime.Reply{Error: err})
		return
	}
	l.mu.Lock()
	l.cur = state
	l.mu.Unlock()
}

func (l *pendingListener) askNext(ctx context.Context, state *active) error {
	q := state.questions[state.selected]
	question := strings.TrimSpace(q.Question)
	if question == "" {
		return fmt.Errorf("question #%d is empty", state.selected+1)
	}
	header := fmt.Sprintf("(%d/%d) %s", state.selected+1, len(state.questions), question)

	switch {
	case len(q.Options) == 0 && q.Secret:
		msg, err := l.bot.client.SendInput(ctx, state.chatID, 0, header+"\n\n🔒 Your reply will be deleted from chat after capture.")
		if err != nil {
			return fmt.Errorf("SendInput: %w", err)
		}
		state.kind = typeAskSecret
		state.message = msg.ID

	case len(q.Options) == 0:
		msg, err := l.bot.client.SendInput(ctx, state.chatID, 0, header)
		if err != nil {
			return fmt.Errorf("SendInput: %w", err)
		}
		state.kind = typeAskText
		state.message = msg.ID

	case q.MultiSelect:
		msg, err := l.bot.client.SendMultiSelect(ctx, state.chatID, 0, header, q.Options)
		if err != nil {
			return fmt.Errorf("SendMultiSelect: %w", err)
		}
		state.kind = typeAskMulti
		state.message = msg.ID

	default:
		msg, err := l.bot.client.SendSelect(ctx, state.chatID, 0, header, q.Options)
		if err != nil {
			return fmt.Errorf("SendSelect: %w", err)
		}
		state.kind = typeAskSingle
		state.message = msg.ID
	}
	return nil
}

func (l *pendingListener) onCallback(ctx context.Context, chatID int64, msgID int, data string, picks []string) bool {
	l.mu.Lock()
	state := l.cur
	if state == nil || state.chatID != chatID || state.message != msgID {
		l.mu.Unlock()
		return false
	}
	l.mu.Unlock()

	switch state.kind {
	case typeToolConfirm:
		reply, finalize := confirmReplyFor(data)
		if !finalize {
			return true
		}
		l.deletePrompt(ctx, state)
		l.finalize(state, reply)

	case typeAskSingle:
		l.deletePrompt(ctx, state)
		state.answers = append(state.answers, data)
		l.advance(ctx, state)

	case typeAskMulti:
		l.deletePrompt(ctx, state)
		state.answers = append(state.answers, picks)
		l.advance(ctx, state)

	default:
		return false
	}
	return true
}

func (l *pendingListener) onText(ctx context.Context, chatID int64, msgID int, text string) bool {
	l.mu.Lock()
	state := l.cur
	if state == nil || state.chatID != chatID {
		l.mu.Unlock()
		return false
	}
	if state.kind != typeAskText && state.kind != typeAskSecret {
		l.mu.Unlock()
		return false
	}
	l.mu.Unlock()

	if err := l.bot.client.Delete(ctx, chatID, msgID); err != nil {
		slog.Warn("pendingListener.Delete user reply",
			slog.Int64("chat", chatID),
			slog.Int("msg", msgID),
			slog.Bool("secret", state.kind == typeAskSecret),
			slog.String("error", err.Error()))
	}
	l.deletePrompt(ctx, state)
	state.answers = append(state.answers, strings.TrimRight(text, "\r\n"))
	l.advance(ctx, state)
	return true
}

func (l *pendingListener) deletePrompt(ctx context.Context, state *active) {
	if state.message == 0 {
		return
	}
	if err := l.bot.client.Delete(ctx, state.chatID, state.message); err != nil {
		slog.Warn("pendingListener.Delete prompt",
			slog.Int64("chat", state.chatID),
			slog.Int("msg", state.message),
			slog.String("error", err.Error()))
	}
	state.message = 0
}

func (l *pendingListener) advance(ctx context.Context, state *active) {
	state.selected++
	if state.selected >= len(state.questions) {
		l.finalize(state, runtime.Reply{Answers: state.answers})
		return
	}
	if err := l.askNext(ctx, state); err != nil {
		l.finalize(state, runtime.Reply{Error: err})
	}
}

func (l *pendingListener) finalize(state *active, reply runtime.Reply) {
	l.mu.Lock()
	if l.cur == state {
		l.cur = nil
	}
	l.mu.Unlock()
	runtime.Resolve(state.pendingID, reply)
	select {
	case l.wakeup <- struct{}{}:
	default:
	}
}

var confirmOptions = []string{
	"✅ Yes",
	"✅ Yes, don't ask again",
	"❌ No",
	"⛔ Stop",
}

func confirmReplyFor(data string) (reply runtime.Reply, finalize bool) {
	switch data {
	case "✅ Yes":
		return runtime.Reply{Approve: true}, true
	case "✅ Yes, don't ask again":
		return runtime.Reply{Approve: true, Remember: true}, true
	case "❌ No":
		return runtime.Reply{Approve: false, Skip: true}, true
	case "⛔ Stop":
		return runtime.Reply{Approve: false, Error: fmt.Errorf("user stopped")}, true
	default:
		return runtime.Reply{}, false
	}
}
