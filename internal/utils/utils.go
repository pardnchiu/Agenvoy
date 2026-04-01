package utils

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func NewID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
}

func EventLog(tag string, event agentTypes.Event, sessionID string, input string) {
	sessionLog := sessionID
	if len(sessionLog) > 16 {
		sessionLog = sessionLog[:13] + "…"
	}

	if input != "" {
		inputLog := input
		if len(inputLog) > 32 {
			inputLog = inputLog[:31] + "…"
		}
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("input", inputLog))
		return
	}

	switch event.Type {
	case agentTypes.EventSkillSelect, agentTypes.EventAgentSelect:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()))

	case agentTypes.EventSkillResult:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("skill", event.Text))

	case agentTypes.EventAgentResult:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("agent", event.Text))

	case agentTypes.EventToolCall:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("tool", event.ToolName))

	case agentTypes.EventText:
		text := event.Text
		if len(text) > 32 {
			text = text[:31] + "…"
		}
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("output", text))

	case agentTypes.EventError, agentTypes.EventExecError:
		slog.Error(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("error", event.Err.Error()))

	default:
		break
	}
}

func GET[T any](ctx context.Context, client *http.Client, api string, header map[string]string) (T, int, error) {
	var result T

	if client == nil {
		client = &http.Client{}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", api, nil)
	if err != nil {
		return result, 0, err
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return result, 0, err
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode

	if s, ok := any(&result).(*string); ok {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, statusCode, err
		}
		*s = string(b)
		return result, statusCode, nil
	}

	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "xml"):
		err = xml.NewDecoder(resp.Body).Decode(&result)
	default:
		err = json.NewDecoder(resp.Body).Decode(&result)
	}
	if err != nil {
		return result, statusCode, err
	}
	return result, statusCode, nil
}

func POST[T any](ctx context.Context, client *http.Client, api string, header map[string]string, body map[string]any, contentType string) (T, int, error) {
	var result T

	if contentType == "" {
		contentType = "json"
	}

	var req *http.Request
	var err error
	if contentType == "form" {
		requestBody := url.Values{}
		for k, v := range body {
			requestBody.Set(k, fmt.Sprint(v))
		}

		req, err = http.NewRequestWithContext(ctx, "POST", api, strings.NewReader(requestBody.Encode()))
		if err != nil {
			return result, 0, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		requestBody, err := json.Marshal(body)
		if err != nil {
			return result, 0, fmt.Errorf("failed to marshal body: %w", err)
		}

		req, err = http.NewRequestWithContext(ctx, "POST", api, strings.NewReader(string(requestBody)))
		if err != nil {
			return result, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")

	for k, v := range header {
		req.Header.Set(k, v)
	}

	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, 0, fmt.Errorf("failed to send: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode

	if statusCode < 200 || statusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return result, statusCode, fmt.Errorf("HTTP %d: %s", statusCode, strings.TrimSpace(string(b)))
	}

	if s, ok := any(&result).(*string); ok {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, statusCode, fmt.Errorf("failed to read: %w", err)
		}
		*s = string(b)
		return result, statusCode, nil
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, statusCode, fmt.Errorf("failed to read: %w", err)
	}
	return result, statusCode, nil
}

func FormatInt(number int) string {
	s := fmt.Sprintf("%d", number)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
