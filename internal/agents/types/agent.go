package types

import (
	"context"
	"encoding/json"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
)

type Agent = go_llm_router.Agent

type sessionIDCtxKey struct{}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDCtxKey{}, sessionID)
}

func SessionIDFrom(ctx context.Context) string {
	sid, _ := ctx.Value(sessionIDCtxKey{}).(string)
	return sid
}

type windowHashCtxKey struct{}

func WithWindowHash(ctx context.Context, hash string) context.Context {
	return context.WithValue(ctx, windowHashCtxKey{}, hash)
}

func WindowHash(ctx context.Context) string {
	hash, _ := ctx.Value(windowHashCtxKey{}).(string)
	return hash
}

type originCtxKey struct{}

func WithOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, originCtxKey{}, origin)
}

func OriginFrom(ctx context.Context) string {
	origin, _ := ctx.Value(originCtxKey{}).(string)
	return origin
}

type deliverToCtxKey struct{}

func WithDeliverTo(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, deliverToCtxKey{}, sessionID)
}

func DeliverToFrom(ctx context.Context) string {
	sid, _ := ctx.Value(deliverToCtxKey{}).(string)
	return sid
}

type noPromptCtxKey struct{}

func WithNoPrompt(ctx context.Context) context.Context {
	return context.WithValue(ctx, noPromptCtxKey{}, true)
}

func NoPromptFrom(ctx context.Context) bool {
	v, _ := ctx.Value(noPromptCtxKey{}).(bool)
	return v
}

type ModelSet struct {
	Registry map[string]Agent
	Order    []ModelEntry
	Fallback Agent
}

type ModelEntry struct {
	Name string `json:"name"`
}

func (a *ModelEntry) UnmarshalJSON(raw []byte) error {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		a.Name = name
		return nil
	}
	var legacy struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	a.Name = legacy.Name
	return nil
}

type Session struct {
	ID               string
	SystemPrompts    []go_llm_router.Message
	Summary          go_llm_router.Message
	TurnContext      go_llm_router.Message
	UserMessage      go_llm_router.Message
	ToolHistories    []go_llm_router.Message
	ToolResults      []go_llm_router.Message
	RawHistories     []go_llm_router.Message
	ContextHistories []go_llm_router.Message
	Sender           string
	SendAt           int64
	Stateless        bool
}

func ClaudeCodeEnabled() bool {
	return claudeCode.CheckBinary() == nil
}
