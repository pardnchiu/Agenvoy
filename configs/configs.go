package configs

import (
	_ "embed"
)

// * Prompts

//go:embed prompts/agent_selector.md
var AgentSelector string

//go:embed prompts/skill_execution.md
var SkillExecution string

//go:embed prompts/compact_exec_prompt.md
var CompactExecPrompt string

//go:embed prompts/compact_history_prompt.md
var CompactHistoryPrompt string

//go:embed prompts/summary_prompt.md
var SummaryPrompt string

//go:embed prompts/summary_context.md
var SummaryContext string

//go:embed prompts/system_prompt.md
var SystemPrompt string

//go:embed prompts/chatcompletions_system_prompt.md
var ChatCompletionsSystemPrompt string

//go:embed prompts/discord_system_prompt.md
var DiscordSystemPrompt string

//go:embed prompts/discord_format.md
var DiscordFormat string

//go:embed prompts/telegram_system_prompt.md
var TelegramSystemPrompt string

//go:embed prompts/telegram_format.md
var TelegramFormat string

//go:embed prompts/default_session_prompt.md
var DefaultSessionPrompt string

//go:embed prompts/always_allow.md
var PermissionAlwaysAllow string

//go:embed prompts/single_confirm.md
var PermissionSingleConfirm string

//go:embed prompts/tool_guide.md
var ToolGuide string

// * Configs

//go:embed jsons/denied_map.json
var DeniedMap []byte

//go:embed jsons/exclude_list.json
var ExcludeList []byte

//go:embed jsons/white_list.json
var WhiteList []byte

//go:embed jsons/net_white_list.json
var NetWhiteList []byte

//go:embed jsons/tui_tools.json
var TUITools []byte

