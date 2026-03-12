package configs

import (
	_ "embed"
)

//go:embed prompts/agentSelector.md
var AgentSelector string

//go:embed prompts/SkillSelector.md
var SkillSelector string

//go:embed prompts/skillExecution.md
var SkillExecution string

//go:embed prompts/summaryPrompt.md
var SummaryPrompt string

//go:embed prompts/systemPrompt.md
var SystemPrompt string
