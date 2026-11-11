package exec

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

const (
	baseGuideKey      = "_base"
	unlistedGuideKey  = "_base_unlisted"
	vendorGuidePrefix = "_vendor_"
	replyLangAuto     = "user's language, else English; Chinese → 繁體中文（台灣用語）."
)

var guardrailRules = loadGuardrailRules()

func loadGuardrailRules() string {
	var list []string
	if err := json.Unmarshal(configs.GuardrailRules, &list); err != nil {
		slog.Warn("embedded guardrail_rules",
			slog.String("error", err.Error()))
		return ""
	}
	lines := make([]string, 0, len(list))
	for _, rule := range list {
		if rule = strings.TrimSpace(rule); rule != "" {
			lines = append(lines, "- "+rule)
		}
	}
	return strings.Join(lines, "\n")
}

func BuildSystemPrompts(workDir, extraSystemPrompt string, sessionID string, model string) []go_llm_router.Message {
	var prompts []go_llm_router.Message
	if channel := channelSystemPrompt(sessionID); channel != "" {
		prompts = append(prompts, go_llm_router.Message{Role: "system", Content: channel})
	}
	prompts = append(prompts, go_llm_router.Message{Role: "system", Content: getSystemPrompt(workDir, extraSystemPrompt, sessionID, model)})
	return prompts
}

func channelSystemPrompt(sessionID string) string {
	switch {
	case strings.HasPrefix(sessionID, "tg-"):
		return configs.TelegramSystemPrompt
	case strings.HasPrefix(sessionID, "dc-"):
		return configs.DiscordSystemPrompt
	case strings.HasPrefix(sessionID, "ln-"):
		return configs.LineSystemPrompt
	}
	return ""
}

func figureScope(sessionID string) string {
	if strings.HasPrefix(sessionID, "chat-") {
		return "chat replies and `.md` files"
	}
	return "`.md` files only (chat replies in this channel show them as raw source, so they never carry them)"
}

func mcpInstructionsSection() string {
	instructions := mcp.Manager().Instructions()
	if len(instructions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## MCP Server Instructions\n\nEach block is the operating contract its server declared; follow it for that server's tools.\n")
	for _, name := range slices.Sorted(maps.Keys(instructions)) {
		sb.WriteString("\n### ")
		sb.WriteString(name)
		sb.WriteString("\n\n")
		sb.WriteString(instructions[name])
		sb.WriteString("\n")
	}
	sb.WriteString("\n---\n\n")
	return sb.String()
}

func getSystemPrompt(workDir string, extraSystemPrompt string, sessionID string, model string) string {
	systemOS := getSystemInfo().os
	extraSection := strings.TrimSpace(extraSystemPrompt)
	if extraSection != "" {
		extraSection = "## Additional Instructions\n\n" + extraSection + "\n\n---\n\n"
	}

	personaSection := ""
	if sessionID != "" {
		if err := configbot.Save(sessionID, "", "", false); err != nil {
			slog.Debug("sessionBot Save",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}
	if selfID, _, body := configbot.GetPersona(sessionID); body != "" {
		var sb strings.Builder
		sb.WriteString("## Bot Persona\n\n")
		if selfID != "" {
			fmt.Fprintf(&sb, "Your operating identity for this session is `%s`. Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n", selfID)
		} else {
			sb.WriteString("Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n")
		}
		sb.WriteString(body)
		sb.WriteString("\n\n---\n\n")
		personaSection = sb.String()
	}

	return strings.NewReplacer(
		"{{.SystemOS}}", systemOS,
		"{{.OutputDir}}", filesystem.OutputDir(),
		"{{.HostNote}}", hostNoteSection(),
		"{{.ReplyLanguage}}", replyLanguage(),
		"{{.BotPersona}}", personaSection,
		"{{.MCPInstructions}}", mcpInstructionsSection(),
		"{{.OfficialGuide}}", officialGuideSection(model),
		"{{.GuardrailRules}}", guardrailRules,
		"{{.ExtraSystemPrompt}}", extraSection,
		"{{.FigureScope}}", figureScope(sessionID),
	).Replace(configs.SystemPrompt)
}

func replyLanguage() string {
	if directive := filesystem.ReplyLangDirective(); directive != "" {
		return directive
	}
	return replyLangAuto
}

func guideKeyMatches(model, key string) bool {
	if strings.Contains(model, key) {
		return true
	}
	if !strings.HasPrefix(key, "claude") {
		return false
	}
	return strings.Contains(strings.ReplaceAll(model, ".", "-"), strings.ReplaceAll(key, ".", "-"))
}

func officialGuideSection(model string) string {
	if cfg, err := config.Load(); err == nil && cfg.OfficialGuideOff {
		return ""
	}

	matched, vendor := "", ""

	keys := slices.SortedFunc(maps.Keys(configs.OfficialGuides), func(a, b string) int {
		return len(b) - len(a)
	})
	for _, key := range keys {
		if vendorKey, ok := strings.CutPrefix(key, vendorGuidePrefix); ok {
			if vendor == "" && guideKeyMatches(model, vendorKey) {
				vendor = key
			}
			continue
		}
		if key == baseGuideKey || key == unlistedGuideKey {
			continue
		}
		if matched == "" && guideKeyMatches(model, key) {
			matched = key
		}
	}
	if matched == "" && vendor == "" {
		matched = unlistedGuideKey
	}

	body := mergeGuideSections(
		configs.OfficialGuides[baseGuideKey],
		configs.OfficialGuides[vendor],
		configs.OfficialGuides[matched],
	)
	if body == "" {
		return ""
	}
	return "## Model Guide\n\n" + body + "\n\n---\n\n"
}

func mergeGuideSections(layers ...string) string {
	order := []string{}
	dicItems := map[string][]string{}
	dicSeen := map[string]map[string]bool{}

	for _, layer := range layers {
		heading := ""
		for line := range strings.SplitSeq(layer, "\n") {
			if title, ok := strings.CutPrefix(line, "## "); ok {
				heading = strings.TrimSpace(title)
				if _, ok := dicItems[heading]; !ok {
					order = append(order, heading)
					dicItems[heading] = nil
					dicSeen[heading] = map[string]bool{}
				}
				continue
			}
			if line = strings.TrimSpace(line); line == "" || heading == "" || dicSeen[heading][line] {
				continue
			}
			dicSeen[heading][line] = true
			dicItems[heading] = append(dicItems[heading], line)
		}
	}

	builder := strings.Builder{}
	for _, heading := range order {
		if len(dicItems[heading]) == 0 {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("## ")
		builder.WriteString(heading)
		builder.WriteString("\n\n")
		for i, item := range dicItems[heading] {
			if i > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(item)
		}
	}
	return builder.String()
}

func getChatCompletionsSystemPrompt(workDir string, model string) string {
	return strings.NewReplacer(
		"{{.SystemOS}}", getSystemInfo().os,
		"{{.WorkPath}}", workDir,
		"{{.HostNote}}", hostNoteSection(),
		"{{.ReplyLanguage}}", replyLanguage(),
		"{{.MCPInstructions}}", mcpInstructionsSection(),
		"{{.OfficialGuide}}", officialGuideSection(model),
		"{{.GuardrailRules}}", guardrailRules,
	).Replace(configs.ChatCompletionsSystemPrompt)
}

func BuildChatCompletionsSystemPrompts(workDir string, model string) []go_llm_router.Message {
	return []go_llm_router.Message{{Role: "system", Content: getChatCompletionsSystemPrompt(workDir, model)}}
}

func skillListBlock(scanner *runtime.SkillScanner, excludeSkills []string) string {
	if scanner == nil {
		return ""
	}
	names := scanner.List()
	if len(names) == 0 {
		return ""
	}

	excluded := make(map[string]bool, len(excludeSkills))
	for _, n := range excludeSkills {
		excluded[strings.TrimSpace(n)] = true
	}

	var b strings.Builder
	for _, n := range names {
		if excluded[n] {
			continue
		}
		desc := go_pkg_utils.TruncateString(scanner.Skills.ByName[n].Description, 256)
		b.WriteString("- ")
		b.WriteString(n)
		if desc != "" {
			b.WriteString(": ")
			b.WriteString(desc)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderActivation(s *skill.Skill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "active skill: %s\nskill directory: %s\n\n---\n\n", s.Name, s.Path)
	if ext := strings.TrimSpace(configs.SkillExecution); ext != "" {
		b.WriteString(ext)
		b.WriteString("\n\n---\n\n")
	}
	if names := toolregister.BuiltinNames(); len(names) > 0 {
		b.WriteString("### Built-in Tools\n\n")
		for _, name := range names {
			b.WriteString("- `")
			b.WriteString(name)
			b.WriteString("`\n")
		}
		b.WriteString("\n---\n\n")
	}
	b.WriteString(s.Resolved())
	return b.String()
}
