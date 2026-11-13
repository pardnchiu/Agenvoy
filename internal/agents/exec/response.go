package exec

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func Response(content string) string {
	// * remove system prefix
	content = configs.MESSAGE_PREFIX_REGEX.ReplaceAllString(content, "")
	if loc := configs.SUMMARY_LEAK_MARKER_REGEX.FindStringIndex(content); loc != nil {
		dropped := []rune(strings.TrimSpace(content[loc[0]:]))
		head := dropped[:min(len(dropped), configs.LOG_HEAD_RUNES)]
		content = strings.TrimRight(content[:loc[0]], " \t\n\r#")
		slog.Debug("response summary leak",
			slog.String("dropped_head", string(head)),
			slog.Int("dropped_chars", len(dropped)))
	}
	return strings.TrimSpace(content)
}

func extractThinkTag(content string) (think, rest string) {
	loc := configs.THINK_TAG_REGEX.FindStringSubmatchIndex(content)
	if loc == nil {
		return "", strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[loc[2]:loc[3]]), strings.TrimSpace(content[loc[1]:])
}

func guardrailRefusal(sessionID, model, content string) string {
	rule, command := guardrailRule(content)
	runes := []rune(content)
	head := runes[:min(len(runes), configs.LOG_HEAD_RUNES)]
	slog.Debug("refusal",
		slog.String("session", sessionID),
		slog.String("model", model),
		slog.String("rule", rule),
		slog.String("head", string(head)))

	if rule == "secret-exfil" && command != "" {
		return fmt.Sprintf(filesystem.RefusalMessage(rule), command)
	}
	refusal := filesystem.RefusalMessage("default")
	if rule == "" {
		return refusal
	}
	return refusal + " (" + rule + ")"
}

func guardrailRule(content string) (rule, command string) {
	_, after, ok := strings.Cut(content, configs.BAN_TAG)
	if !ok {
		return "", ""
	}

	line, _, _ := strings.Cut(strings.TrimSpace(after), "\n")
	rule, rest, _ := strings.Cut(line, " ")
	rule = strings.Trim(strings.TrimSpace(rule), "[](){}:,.\"'`")
	rest = strings.Trim(strings.TrimSpace(rest), "`")
	if runes := []rune(rest); len(runes) > 300 {
		rest = ""
	}
	return rule, strings.ReplaceAll(rest, "`", "'")
}
