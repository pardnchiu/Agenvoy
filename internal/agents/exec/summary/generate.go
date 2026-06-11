package agentSummary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
)

var (
	fencedBlockRegex    = regexp.MustCompile("(?s)" + "```" + `(?:json|summary)\s*\n([\s\S]*?)\s*\n` + "```")
	summaryTagRegex     = regexp.MustCompile(`(?s)<summary>\s*([\s\S]*?)\s*</summary>`)
	summaryBracketRegex = regexp.MustCompile(`(?s)\[summary\]\s*([\s\S]*?)\s*\[/summary\]`)
)

func Generate(ctx context.Context, agent agentTypes.Agent, sessionID string, histories []agentTypes.Message) error {
	raw, summaryMap := summary.Ensure(sessionID)
	meta := summary.GetMeta(sessionID)

	if meta.LastMessageTime == "" && len(summaryMap) > 0 {
		latest := latestTime(histories)
		summary.SaveMeta(sessionID, latest)
		summary.Save(sessionID, summaryMap)
		return nil
	}

	newHistories := filterAfter(histories, meta.LastMessageTime)
	if len(newHistories) == 0 {
		return nil
	}

	oldSummary := string(raw)
	if oldSummary == "" {
		oldSummary = "{}"
	}

	chunks := chunkMessages(newHistories, 16)
	cursor := meta.LastMessageTime

	for i, chunk := range chunks {
		newMap := generate(ctx, agent, oldSummary, chunk)
		if newMap == nil {
			slog.Warn("summary generatePass returned nil",
				slog.String("session", sessionID),
				slog.Int("chunk", i+1))
			return fmt.Errorf("generatePass returned nil at chunk %d/%d", i+1, len(chunks))
		}

		if raw, err := json.Marshal(newMap); err == nil {
			oldSummary = string(raw)
		}

		summary.Save(sessionID, newMap)
		if chunkLatest := latestTime(chunk); chunkLatest > cursor {
			cursor = chunkLatest
		}
		summary.SaveMeta(sessionID, cursor)
	}
	return nil
}

func chunkMessages(messages []agentTypes.Message, size int) [][]agentTypes.Message {
	if len(messages) == 0 {
		return nil
	}

	var list [][]agentTypes.Message
	for i := 0; i < len(messages); i += size {
		end := min(i+size, len(messages))
		if end < len(messages) && end > 0 {
			last := messages[end-1]
			if last.Role == "user" && end < len(messages) {
				end++
			}
		}
		list = append(list, messages[i:end])
	}
	return list
}

func generate(ctx context.Context, agent agentTypes.Agent, oldSummary string, histories []agentTypes.Message) map[string]any {
	prompt := strings.NewReplacer(
		"{{.Summary}}", oldSummary,
	).Replace(strings.TrimSpace(configs.SummaryPrompt))

	var sb strings.Builder
	for _, hist := range histories {
		str, ok := hist.Content.(string)
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "[%s]\n%s\n\n", hist.Role, str)
	}

	messages := []agentTypes.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: "```\n" + sb.String() + "```\n\nGenerate the updated summary now per the rules above. Output raw JSON only."},
	}

	return exec(ctx, agent, messages)
}

func exec(ctx context.Context, agent agentTypes.Agent, messages []agentTypes.Message) map[string]any {
	resp, err := agent.Send(ctx, messages, nil)
	if err != nil {
		slog.Warn("agentTypes.Agent Send",
			slog.String("error", err.Error()))
		return nil
	}
	if len(resp.Choices) == 0 {
		return nil
	}

	str, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		slog.Warn("agentTypes.Agent Send: non-string content",
			slog.String("type", fmt.Sprintf("%T", resp.Choices[0].Message.Content)))
		return nil
	}

	str = strings.TrimSpace(str)
	if str == "" {
		return nil
	}

	var dic map[string]any
	for _, regex := range []*regexp.Regexp{fencedBlockRegex, summaryTagRegex, summaryBracketRegex} {
		if match := regex.FindStringSubmatchIndex(str); match != nil {
			part := str[match[2]:match[3]]
			if json.Unmarshal([]byte(part), &dic) == nil && summary.IsValid(dic) {
				return dic
			}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader([]byte(str)))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		if delim, ok := token.(json.Delim); ok && delim == '{' {
			start := strings.Index(str, "{")
			if start == -1 {
				break
			}
			var dic map[string]any
			if json.Unmarshal([]byte(str[start:]), &dic) == nil && summary.IsValid(dic) {
				return dic
			}
			break
		}
	}

	slog.Warn("agentTypes.Agent Send: unparseable",
		slog.String("preview", go_pkg_utils.TruncateString(str, 256)))
	return nil
}

func latestTime(messages []agentTypes.Message) string {
	var str string
	for _, message := range messages {
		t := extractTime(message)
		if t > str {
			str = t
		}
	}
	return str
}

func extractTime(msg agentTypes.Message) string {
	str, ok := msg.Content.(string)
	if !ok {
		return ""
	}
	list := regexp.MustCompile(`當前時間:\s*(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`).FindStringSubmatch(str)
	if len(list) < 2 {
		return ""
	}
	return list[1]
}

func filterAfter(messages []agentTypes.Message, cursor string) []agentTypes.Message {
	if cursor == "" {
		return messages
	}

	list := make([]agentTypes.Message, 0, len(messages))
	for _, message := range messages {
		t := extractTime(message)
		if t == "" || t > cursor {
			list = append(list, message)
		}
	}
	return list
}
