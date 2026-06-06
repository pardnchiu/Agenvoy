package exec

import (
	"fmt"
	"log/slog"
	goRuntime "runtime"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	toolSearcher "github.com/pardnchiu/agenvoy/internal/tools/searcher"
)

func BuildSystemPrompts(workDir, extraSystemPrompt string, scanner *runtime.SkillScanner, sessionID string, allowAll, webMode bool, excludeSkills []string) []agentTypes.Message {
	var prompts []agentTypes.Message
	switch {
	case strings.HasPrefix(sessionID, "tg-"):
		prompts = append(prompts, agentTypes.Message{Role: "system", Content: configs.TelegramSystemPrompt})
	case strings.HasPrefix(sessionID, "dc-"):
		prompts = append(prompts, agentTypes.Message{Role: "system", Content: configs.DiscordSystemPrompt})
	}
	prompts = append(prompts, agentTypes.Message{Role: "system", Content: getSystemPrompt(workDir, extraSystemPrompt, scanner, sessionID, allowAll, webMode, excludeSkills)})
	return prompts
}

func getSystemPrompt(workDir string, extraSystemPrompt string, scanner *runtime.SkillScanner, sessionID string, allowAll bool, webMode bool, excludeSkills []string) string {
	systemOS := goRuntime.GOOS
	var extraSection string
	if extra := strings.TrimSpace(extraSystemPrompt); extra != "" {
		extraSection = "---\n\n## Additional Instructions\n\n" + extra + "\n\n---\n\n"
	}

	template := configs.SystemPrompt
	if webMode {
		template = configs.WebModeSystemPrompt
	}

	skillsSection := ""
	if list := toolSearcher.ListBlock(scanner, excludeSkills); list != "" {
		skillsSection = "## Skills\n\n" +
			"**Slash invocations (`/<name>`) are STRICT EXECUTION.** The user has explicitly authorized the skill's full procedure; every step in SKILL.md is binding and must complete via tool calls in order. The FIRST step (often `ask_user` for requirement gathering) must run before any other tool call — no exceptions, no \"the user input looks complete so I'll skip ahead\".\n\n" +
			"The `run_skill` tool path is advisory — consult, integrate parts that fit, ignore parts that don't. Consider activating a skill when its description matches the user's intent on each turn, even without an explicit `/<name>` invocation.\n\n" +
			list
	}

	personaSection := ""
	if sessionID != "" {
		if err := configBot.Save(sessionID, "", "", false); err != nil {
			slog.Warn("sessionBot Save",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}
	if name, body := configBot.Get(sessionID); body != "" {
		var sb strings.Builder
		sb.WriteString("## Bot Persona\n\n")
		if name != "" {
			fmt.Fprintf(&sb, "Your operating identity for this session is `%s`. Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n", name)
		} else {
			sb.WriteString("Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n")
		}
		sb.WriteString(body)
		sb.WriteString("\n\n---\n\n")
		personaSection = sb.String()
	}

	return strings.NewReplacer(
		"{{.SystemOS}}", systemOS,
		"{{.WorkPath}}", workDir,
		"{{.BotPersona}}", personaSection,
		"{{.PermissionMode}}", buildPermissionModeSection(allowAll),
		"{{.AvailableSkills}}", skillsSection,
		"{{.ExternalAgents}}", buildExternalAgentsPrompt(),
		"{{.CrossChannelSending}}", buildCrossChannelPrompt(),
		"{{.ExtraSystemPrompt}}", extraSection,
	).Replace(template)
}

func buildPermissionModeSection(allowAll bool) string {
	if allowAll {
		return strings.TrimRight(configs.PermissionAlwaysAllow, "\n")
	}
	return strings.TrimRight(configs.PermissionSingleConfirm, "\n")
}

func getChatCompletionsSystemPrompt(workDir string, scanner *runtime.SkillScanner, excludeSkills []string) string {
	skillsSection := ""
	if list := toolSearcher.ListBlock(scanner, excludeSkills); list != "" {
		skillsSection = "## Skills\n\n" +
			"**Slash invocations (`/<name>`) are STRICT EXECUTION.** The user has explicitly authorized the skill's full procedure; every step in SKILL.md is binding and must complete via tool calls in order. The FIRST step (often `ask_user` for requirement gathering) must run before any other tool call — no exceptions, no \"the user input looks complete so I'll skip ahead\".\n\n" +
			"The `run_skill` tool path is advisory — consult, integrate parts that fit, ignore parts that don't. Consider activating a skill when its description matches the user's intent on each turn, even without an explicit `/<name>` invocation.\n\n" +
			list
	}

	return strings.NewReplacer(
		"{{.SystemOS}}", goRuntime.GOOS,
		"{{.WorkPath}}", workDir,
		"{{.AvailableSkills}}", skillsSection,
	).Replace(configs.ChatCompletionsSystemPrompt)
}

func BuildChatCompletionsSystemPrompts(workDir string, scanner *runtime.SkillScanner, excludeSkills []string) []agentTypes.Message {
	return []agentTypes.Message{{Role: "system", Content: getChatCompletionsSystemPrompt(workDir, scanner, excludeSkills)}}
}
