package tui

import (
	"fmt"
	"strings"
)

func activityVerb(activity string) string {
	switch {
	case activity == "":
		return "Thinking"

	case activity == "responding":
		return "Responsing"

	case activity == "selecting agent...":
		return "Selecting agent"

	case activity == "summarizing...":
		return "Summarizing"

	case strings.HasPrefix(activity, "tool: "):
		tool := strings.TrimPrefix(activity, "tool: ")
		switch tool {
		case "read_files":
			return "Reading"

		case "edit_file":
			return "Writing"

		case "run_command", "run_command_background":
			return "Running"

		case "search_web", "error_history", "chat_history":
			return "Searching"

		case "fetch_page":
			return "Fetching"

		case "subagents":
			return "Delegating"

		case "find_files":
			return "Listing"

		case "calculate":
			return "Calculating"

		case "run_skill":
			return "Activating skill"
		}
		return tool
	}
	return "Thinking"
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
