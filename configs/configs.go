package configs

import (
	_ "embed"
)

// * Prompts

//go:embed prompts/agent_selector.md
var AgentSelector string

//go:embed prompts/skill_execution.md
var SkillExecution string

//go:embed prompts/summary_prompt.md
var SummaryPrompt string

//go:embed prompts/summary_context.md
var SummaryContext string

//go:embed prompts/summary_merge_prompt.md
var SummaryMergePrompt string

//go:embed prompts/system_prompt.md
var SystemPrompt string

//go:embed prompts/discord_system_prompt.md
var DiscordSystemPrompt string

// * Configs

//go:embed jsons/denied_map.json
var DeniedMap []byte

//go:embed jsons/exclude_list.json
var ExcludeList []byte

//go:embed jsons/white_list.json
var WhiteList []byte

// * Providers

//go:embed jsons/providors/claude.json
var ClaudeModels []byte

//go:embed jsons/providors/copilot.json
var CopilotModels []byte

//go:embed jsons/providors/gemini.json
var GeminiModels []byte

//go:embed jsons/providors/nvidia.json
var NvidiaModels []byte

//go:embed jsons/providors/codex.json
var CodexModels []byte

//go:embed jsons/providors/openai.json
var OpenaiModels []byte
