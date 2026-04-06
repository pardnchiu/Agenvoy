package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func GenerateSummary(ctx context.Context, agent agentTypes.Agent, sessionID string, histories []agentTypes.Message) {
	raw, _ := sessionManager.EnsureSummary(sessionID)

	var oldSummary string
	if raw != nil {
		oldSummary = string(raw)
	} else {
		oldSummary = "{}"
	}

	chunks := chunkMessages(histories, 16)

	for i, chunk := range chunks {
		newMap := generatePass(ctx, agent, oldSummary, chunk)
		if newMap == nil {
			slog.Warn("summary generatePass returned nil",
				slog.String("session", sessionID),
				slog.Int("chunk", i+1))
			continue
		}

		if oldSummary != "{}" {
			newJSON, err := json.Marshal(newMap)
			if err == nil {
				merged := mergePass(ctx, agent, oldSummary, string(newJSON))
				if merged != nil {
					newMap = merged
				}
			}
		}

		if b, err := json.Marshal(newMap); err == nil {
			oldSummary = string(b)
		}

		sessionManager.SaveSummary(sessionID, newMap)
	}
}

func chunkMessages(messages []agentTypes.Message, size int) [][]agentTypes.Message {
	if len(messages) == 0 {
		return nil
	}
	var chunks [][]agentTypes.Message
	for i := 0; i < len(messages); i += size {
		end := i + size
		if end > len(messages) {
			end = len(messages)
		}
		chunks = append(chunks, messages[i:end])
	}
	return chunks
}

func generatePass(ctx context.Context, agent agentTypes.Agent, oldSummary string, histories []agentTypes.Message) map[string]any {
	systemContent := strings.NewReplacer(
		"{{.Summary}}", oldSummary,
	).Replace(strings.TrimSpace(configs.SummaryPrompt))

	messages := []agentTypes.Message{
		{Role: "system", Content: systemContent},
	}
	for _, h := range histories {
		if h.Role != "user" && h.Role != "assistant" {
			continue
		}
		s, ok := h.Content.(string)
		if !ok {
			continue
		}
		messages = append(messages, agentTypes.Message{Role: h.Role, Content: s})
	}

	return sendAndParse(ctx, agent, messages, "generatePass")
}

func mergePass(ctx context.Context, agent agentTypes.Agent, oldSummary, newSummary string) map[string]any {
	prompt := strings.NewReplacer(
		"{{.OldSummary}}", oldSummary,
		"{{.NewSummary}}", newSummary,
	).Replace(strings.TrimSpace(configs.SummaryMergePrompt))

	messages := []agentTypes.Message{
		{Role: "system", Content: prompt},
	}

	return sendAndParse(ctx, agent, messages, "mergePass")
}

func sendAndParse(ctx context.Context, agent agentTypes.Agent, messages []agentTypes.Message, label string) map[string]any {
	resp, err := agent.Send(ctx, messages, nil)
	if err != nil {
		slog.Warn(label+" agent.Send",
			slog.String("error", err.Error()))
		return nil
	}
	if len(resp.Choices) == 0 {
		return nil
	}
	text, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var result map[string]any
	for _, re := range []*regexp.Regexp{fencedBlockRegex, summaryTagRegex, summaryBracketRegex} {
		if loc := re.FindStringSubmatchIndex(text); loc != nil {
			part := text[loc[2]:loc[3]]
			if json.Unmarshal([]byte(part), &result) == nil && isSummaryJSON(result) {
				return result
			}
		}
	}

	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if delim, ok := tok.(json.Delim); ok && delim == '{' {
			start := strings.Index(text, "{")
			if start == -1 {
				break
			}
			var m map[string]any
			if json.Unmarshal([]byte(text[start:]), &m) == nil && isSummaryJSON(m) {
				return m
			}
			break
		}
	}
	return nil
}

func GetSummaryHistories(histories []agentTypes.Message) []agentTypes.Message {
	var filtered []agentTypes.Message
	for _, h := range histories {
		if h.Role == "user" || h.Role == "assistant" {
			filtered = append(filtered, h)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
