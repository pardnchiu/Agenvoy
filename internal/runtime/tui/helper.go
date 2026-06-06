package tui

import (
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/session"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func activityVerb(activity string) string {
	switch {
	case activity == "":
		return "Thinking"

	case activity == "responding":
		return "Responsing"

	case activity == "selecting agent…":
		return "Selecting agent"

	case activity == "summarizing…":
		return "Summarizing"

	case strings.HasPrefix(activity, "tool: "):
		tool := strings.TrimPrefix(activity, "tool: ")
		switch tool {
		case "read_file":
			return "Reading"

		case "write_file", "patch_file":
			return "Writing"

		case "run_command":
			return "Running"

		case "search_web", "search_google_news", "search_error_history", "search_chat_history":
			return "Searching"

		case "fetch_page":
			return "Fetching"

		case "invoke_subagent", "invoke_external_agent", "cross_review_with_external_agents":
			return "Delegating"

		case "list_files", "glob_files", "search_content":
			return "Listing"

		case "calculate":
			return "Calculating"

		case "remember_error":
			return "Remembering"

		case "run_skill":
			return "Activating skill"
		}
		return tool
	}
	return "Thinking"
}

func targetSession(input, currentId string) string {
	name, _ := session.CheckAssign(input)
	if name == "" {
		return ""
	}

	id := session.GetSessionID(name)
	if id == "" {
		return name
	}
	if id == currentId {
		return ""
	}

	if bot, _ := configBot.Get(id); strings.TrimSpace(bot) != "" && bot != id {
		return bot
	}
	return utils.ShortenSessionID(id)
}

func formatTime(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh%02dm", secs/3600, (secs%3600)/60)
}
