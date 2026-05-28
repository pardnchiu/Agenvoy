package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type QuestionType int

const (
	TypeToolConfirm QuestionType = iota
	TypeAskText
	TypeAskSecret
	TypeAskSingle
	TypeAskMulti
)

type Transport[CID comparable, MID comparable] interface {
	LookupChatID(sessionID string) (CID, error)
	SendConfirm(ctx context.Context, chatID CID, toolName, toolArgs string, multiline bool) (MID, error)
	SendAskText(ctx context.Context, chatID CID, header string, secret bool) (MID, error)
	SendAskSingle(ctx context.Context, chatID CID, header string, options []string) (MID, error)
	SendAskMulti(ctx context.Context, chatID CID, header string, options []string) (MID, error)
}

type active[CID comparable, MID comparable] struct {
	pendingID string
	chatID    CID
	chatName  string
	message   MID
	kind      QuestionType
	questions []Question
	selected  int
	answers   []any
}

type Listener[CID comparable, MID comparable] struct {
	transport  Transport[CID, MID]
	prefix     string
	lookupName func(CID) string
	delete     func(ctx context.Context, chatID CID, msgID MID) error
	cancelFn   context.CancelFunc
	mu         sync.Mutex
	cur        map[CID]*active[CID, MID]
	wakeup     chan struct{}
	zeroMID    MID
}

func New[CID comparable, MID comparable](
	transport Transport[CID, MID],
	prefix string,
	lookupName func(CID) string,
	deleteMsg func(ctx context.Context, chatID CID, msgID MID) error,
) *Listener[CID, MID] {
	ctx, cancel := context.WithCancel(context.Background())
	var zeroMID MID
	listener := &Listener[CID, MID]{
		transport:  transport,
		prefix:     prefix,
		lookupName: lookupName,
		delete:     deleteMsg,
		cancelFn:   cancel,
		cur:        make(map[CID]*active[CID, MID]),
		wakeup:     make(chan struct{}, 1),
		zeroMID:    zeroMID,
	}
	go listener.loop(ctx)
	return listener
}

func (l *Listener[CID, MID]) Stop() {
	if l == nil {
		return
	}
	l.cancelFn()
}

func (l *Listener[CID, MID]) IsAwaitingChat(chatID CID) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cur[chatID] != nil
}

func (l *Listener[CID, MID]) IsAwaitingPrompt(chatID CID, msgID MID) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.cur[chatID]
	return a != nil && a.message == msgID
}

func (l *Listener[CID, MID]) loop(ctx context.Context) {
	notify, unregister := RegisterListener(l.prefix)
	defer unregister()

	for {
		l.emit(ctx)
		select {
		case <-ctx.Done():
			return
		case <-notify:
		case <-l.wakeup:
		}
	}
}

func (l *Listener[CID, MID]) emit(ctx context.Context) {
	accept := func(r Request) bool {
		chatID, err := l.transport.LookupChatID(r.SessionID)
		if err != nil {
			return true
		}
		l.mu.Lock()
		busy := l.cur[chatID] != nil
		l.mu.Unlock()
		return !busy
	}

	for {
		id, next, ok := PickNextMatch(l.prefix, accept)
		if !ok {
			return
		}
		if next.Ctx != nil {
			if err := next.Ctx.Err(); err != nil {
				Resolve(id, Reply{Error: err})
				continue
			}
		}
		l.sendPrompt(ctx, id, next)
	}
}

func (l *Listener[CID, MID]) sendPrompt(ctx context.Context, id string, req Request) {
	chatID, err := l.transport.LookupChatID(req.SessionID)
	if err != nil {
		slog.Warn("connector.LookupChatID",
			slog.String("prefix", l.prefix),
			slog.String("session", req.SessionID),
			slog.String("error", err.Error()))
		Resolve(id, Reply{Error: fmt.Errorf("LookupChatID: %w", err)})
		return
	}

	switch req.Kind {
	case KindToolConfirm:
		l.startConfirm(ctx, id, chatID, req)
	case KindAskUser:
		l.startAskUser(ctx, id, chatID, req)
	default:
		Resolve(id, Reply{Error: fmt.Errorf("unknown pending kind: %s", req.Kind)})
	}
}

func (l *Listener[CID, MID]) startConfirm(ctx context.Context, id string, chatID CID, req Request) {
	args, multiline := FormatToolArgs(req.ToolName, req.ToolArgs)
	msgID, err := l.transport.SendConfirm(ctx, chatID, req.ToolName, args, multiline)
	if err != nil {
		Resolve(id, Reply{Error: fmt.Errorf("SendConfirm: %w", err)})
		return
	}
	l.mu.Lock()
	l.cur[chatID] = &active[CID, MID]{
		pendingID: id,
		chatID:    chatID,
		chatName:  l.lookupName(chatID),
		message:   msgID,
		kind:      TypeToolConfirm,
	}
	l.mu.Unlock()
}

func (l *Listener[CID, MID]) startAskUser(ctx context.Context, id string, chatID CID, req Request) {
	if req.AskUser == nil || len(req.AskUser.Questions) == 0 {
		Resolve(id, Reply{Error: fmt.Errorf("ask_user with no questions")})
		return
	}
	state := &active[CID, MID]{
		pendingID: id,
		chatID:    chatID,
		chatName:  l.lookupName(chatID),
		questions: req.AskUser.Questions,
		answers:   make([]any, 0, len(req.AskUser.Questions)),
	}
	if err := l.askNext(ctx, state); err != nil {
		Resolve(id, Reply{Error: err})
		return
	}
	l.mu.Lock()
	l.cur[chatID] = state
	l.mu.Unlock()
}

func (l *Listener[CID, MID]) askNext(ctx context.Context, state *active[CID, MID]) error {
	q := state.questions[state.selected]
	question := strings.TrimSpace(q.Question)
	if question == "" {
		return fmt.Errorf("question #%d is empty", state.selected+1)
	}
	header := fmt.Sprintf("(%d/%d) %s", state.selected+1, len(state.questions), question)

	switch {
	case len(q.Options) == 0 && q.Secret:
		msgID, err := l.transport.SendAskText(ctx, state.chatID, header, true)
		if err != nil {
			return fmt.Errorf("SendAskText: %w", err)
		}
		state.kind = TypeAskSecret
		state.message = msgID

	case len(q.Options) == 0:
		msgID, err := l.transport.SendAskText(ctx, state.chatID, header, false)
		if err != nil {
			return fmt.Errorf("SendAskText: %w", err)
		}
		state.kind = TypeAskText
		state.message = msgID

	case q.MultiSelect:
		msgID, err := l.transport.SendAskMulti(ctx, state.chatID, header, q.Options)
		if err != nil {
			return fmt.Errorf("SendAskMulti: %w", err)
		}
		state.kind = TypeAskMulti
		state.message = msgID

	default:
		msgID, err := l.transport.SendAskSingle(ctx, state.chatID, header, q.Options)
		if err != nil {
			return fmt.Errorf("SendAskSingle: %w", err)
		}
		state.kind = TypeAskSingle
		state.message = msgID
	}
	return nil
}

func (l *Listener[CID, MID]) OnCallback(ctx context.Context, chatID CID, msgID MID, data string, picks []string) bool {
	l.mu.Lock()
	state := l.cur[chatID]
	if state == nil {
		l.mu.Unlock()
		slog.Warn("connector.OnCallback miss (no state)",
			slog.String("prefix", l.prefix),
			slog.Any("chat", chatID),
			slog.Any("msg", msgID))
		return false
	}
	if state.message != msgID {
		expected := state.message
		l.mu.Unlock()
		slog.Warn("connector.OnCallback miss (msg mismatch)",
			slog.String("prefix", l.prefix),
			slog.Any("chat", chatID),
			slog.Any("got_msg", msgID),
			slog.Any("expected_msg", expected))
		return false
	}
	l.mu.Unlock()

	switch state.kind {
	case TypeToolConfirm:
		reply, finalize := confirmReplyFor(data)
		if !finalize {
			return true
		}
		l.deletePrompt(ctx, state)
		l.finalize(state, reply)

	case TypeAskSingle:
		l.deletePrompt(ctx, state)
		state.answers = append(state.answers, data)
		l.advance(ctx, state)

	case TypeAskMulti:
		l.deletePrompt(ctx, state)
		state.answers = append(state.answers, picks)
		l.advance(ctx, state)

	case TypeAskText, TypeAskSecret:
		l.deletePrompt(ctx, state)
		state.answers = append(state.answers, strings.TrimRight(data, "\r\n"))
		l.advance(ctx, state)

	default:
		return false
	}
	return true
}

func (l *Listener[CID, MID]) OnText(ctx context.Context, chatID CID, msgID MID, text string) bool {
	l.mu.Lock()
	state := l.cur[chatID]
	if state == nil {
		l.mu.Unlock()
		return false
	}
	if state.kind != TypeAskText && state.kind != TypeAskSecret {
		l.mu.Unlock()
		return false
	}
	secret := state.kind == TypeAskSecret
	l.mu.Unlock()

	if err := l.delete(ctx, chatID, msgID); err != nil {
		slog.Warn("connector.Delete user reply",
			slog.String("prefix", l.prefix),
			slog.String("chat", state.chatName),
			slog.Bool("secret", secret),
			slog.String("error", err.Error()))
	}
	l.deletePrompt(ctx, state)
	state.answers = append(state.answers, strings.TrimRight(text, "\r\n"))
	l.advance(ctx, state)
	return true
}

func (l *Listener[CID, MID]) deletePrompt(ctx context.Context, state *active[CID, MID]) {
	if state.message == l.zeroMID {
		return
	}
	if err := l.delete(ctx, state.chatID, state.message); err != nil {
		slog.Warn("connector.Delete prompt",
			slog.String("prefix", l.prefix),
			slog.String("chat", state.chatName),
			slog.String("error", err.Error()))
	}
	state.message = l.zeroMID
}

func (l *Listener[CID, MID]) advance(ctx context.Context, state *active[CID, MID]) {
	state.selected++
	if state.selected >= len(state.questions) {
		l.finalize(state, Reply{Answers: state.answers})
		return
	}
	if err := l.askNext(ctx, state); err != nil {
		l.finalize(state, Reply{Error: err})
	}
}

func (l *Listener[CID, MID]) finalize(state *active[CID, MID], reply Reply) {
	l.mu.Lock()
	if l.cur[state.chatID] == state {
		delete(l.cur, state.chatID)
	}
	l.mu.Unlock()
	Resolve(state.pendingID, reply)
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

func ConfirmOptions() []string {
	return confirmOptions
}

func confirmReplyFor(data string) (reply Reply, finalize bool) {
	switch data {
	case "✅ Yes":
		return Reply{Approve: true}, true
	case "✅ Yes, don't ask again":
		return Reply{Approve: true, Remember: true}, true
	case "❌ No":
		return Reply{Approve: false, Skip: true}, true
	case "⛔ Stop":
		return Reply{Approve: false, Error: fmt.Errorf("user stopped")}, true
	default:
		return Reply{}, false
	}
}

func FormatToolArgs(name, raw string) (formatted string, multiline bool) {
	if name == "run_command" {
		var parsed struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && len(parsed.Argv) > 0 {
			return strings.Join(parsed.Argv, " "), false
		}
	}
	args := strings.ReplaceAll(raw, "\r", "")
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(args), "", "  "); err == nil {
		return pretty.String(), true
	}
	return args, true
}
