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

	var summarySection string
	if raw != nil {
		summarySection = string(raw)
	} else {
		summarySection = "{}"
	}

	systemContent := strings.NewReplacer(
		"{{.Summary}}", summarySection,
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

	resp, err := agent.Send(ctx, messages, nil)
	if err != nil {
		slog.Warn("GenerateSummary agent.Send",
			slog.String("error", err.Error()))
		return
	}
	if len(resp.Choices) == 0 {
		return
	}
	text, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	var newMap map[string]any
	found := false
	for _, re := range []*regexp.Regexp{fencedBlockRegex, summaryTagRegex, summaryBracketRegex} {
		if loc := re.FindStringSubmatchIndex(text); loc != nil {
			part := text[loc[2]:loc[3]]
			if json.Unmarshal([]byte(part), &newMap) == nil && isSummaryJSON(newMap) {
				found = true
				break
			}
		}
	}

	if !found {
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
					newMap = m
					found = true
				}
				break
			}
		}
	}

	if !found || newMap == nil {
		slog.Warn("GenerateSummary: no valid summary JSON found in response")
		return
	}
	if !isSummaryJSON(newMap) {
		return
	}

	_, oldMap := sessionManager.GetSummary(sessionID)
	if oldMap != nil {
		newMap = mergeSummary(oldMap, newMap)
	}
	sessionManager.SaveSummary(sessionID, newMap)
}

func GetSummaryHistories(histories []agentTypes.Message) []agentTypes.Message {
	if len(histories) == 0 {
		return nil
	}

	cloned := make([]agentTypes.Message, len(histories))
	copy(cloned, histories)
	return cloned
}
