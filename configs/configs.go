package configs

import (
	"embed"
	"regexp"
	"strings"
	"time"
)

const (
	// * App
	APP_NAME           = "Agenvoy"
	TIME_LAYOUT        = "2006-01-02 15:04:05"
	REASONING_AUTO     = "auto"
	BAN_TAG            = "[KARAPPO]"
	DAEMON_LOG_CHANNEL = "daemon"

	// * Context
	COMPACT_THRESHOLD_RATIO = 0.8
	FALLBACK_CONTEXT_WINDOW = 128_000
	SUMMARY_RUNES           = 32_000
	LOG_HEAD_RUNES          = 160

	// * Cache
	TTL_MEMORY_SEC       = 90 * 24 * 60 * 60
	TTL_TOOL_CACHE_SEC   = 30 * 60
	TTL_MODELS_CACHE_SEC = 15 * 60

	// * Concurrency
	MAX_CONCURRENT_SUBAGENTS = 3
	MAX_CONCURRENT_TOOLS     = 5
	MAX_SESSION_TASKS        = 3

	// * Routing
	TIMEOUT_DISPATCH_CALL = 30 * time.Second

	// * Jev
	TIMEOUT_JEV_CALL         = 3 * time.Second
	MAX_JEV_HISTORY_MESSAGES = 4
	MAX_JEV_RUNES            = 2048

	// * Retry
	MAX_RETRY_TIMES             = 3
	HEALTH_CHECK_TIMEOUT        = 10 * time.Second
	SEND_TIMEOUT_RETRY_INTERVAL = 15 * time.Second
	RATE_LIMIT_COOLDOWN         = 30 * time.Minute

	// * Tool
	CONFIRM_TIMEOUT        = 5 * time.Minute
	DEFAULT_TOOL_TIMEOUT   = 15 * time.Minute
	RUN_COMMAND_TIMEOUT    = 30 * time.Minute
	MAX_WATCH_SCRIPT_DEPTH = 4

	// * Followup
	FOLLOWUP_TIMEOUT        = 30 * time.Second
	FOLLOWUP_MAX_TURNS      = 4
	FOLLOWUP_MAX_TURN_RUNES = 512
	FOLLOWUP_MAX_TITLE      = 32
	FOLLOWUP_MAX_SUGGEST    = 3
	FOLLOWUP_SUGGEST_RUNES  = 32

	// * Size
	MAX_DOCUMENT_BYTES       = 1 << 20
	MAX_PENDING_RESULT_BYTES = MAX_DOCUMENT_BYTES / 32
	MAX_PENDING_ARGS_BYTES   = MAX_DOCUMENT_BYTES / 256

	// * Endpoint
	ENDPOINT_LLM_WINDOW    = "https://llm-io.agenvoy.com/"
	ENDPOINT_UPDATE_SHELL  = "https://agenvoy.com/scripts/update.sh"
	ENDPOINT_HTML_TEMPLATE = "https://view.agenvoy.com"
)

var (
	// * Runtime
	MAX_TOOL_ITERATIONS      = 256
	AGENT_SEND_TIMEOUT_SEC   = 10 * 60
	MAX_HISTORY_MESSAGES     = 16
	MAX_HISTORY_BYTES        = MAX_DOCUMENT_BYTES * 4
	MAX_SUBAGENT_TIMEOUT_MIN = 30
	MAX_RESUME_WAIT_MIN      = 60

	MAX_BACKGROUND_SUBAGENTS            = 3
	MAX_BACKGROUND_SUBAGENT_TIMEOUT_MIN = 60

	// * Tools
	SYSTEM_DEFAULT_TOOLS = []string{"find_tools", "run_tool", "reasoning_guide", "write_todo"}

	// * Pattern
	RETRY_INTERVALS = []time.Duration{
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}
	TIME_RANGES = map[string]time.Duration{
		"1d": 24 * time.Hour,
		"7d": 7 * 24 * time.Hour,
		"1m": 30 * 24 * time.Hour,
		"1y": 365 * 24 * time.Hour,
	}
	USAGE_PERIODS = []struct {
		Label string
		Days  int
	}{
		{Label: "24h", Days: 1},
		{Label: "7d", Days: 7},
		{Label: "28d", Days: 28},
	}

	// * Regex
	FRONTMATTER_REGEX         = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	HTML_TAG_REGEX            = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)
	CACHE_HIT_PCT_REGEX       = regexp.MustCompile(`^\((\d+)%\)$`)
	MESSAGE_PREFIX_REGEX      = regexp.MustCompile(`\A\s*(?:sendAt|sender|channelId)\s*:[^\n]*(?:\n|\z)`)
	SUMMARY_LEAK_MARKER_REGEX = regexp.MustCompile(`(?i)(?:Prior Conversation Context|Prior summary|"key_decisions"\s*:\s*\[|"current_discussion"\s*:\s*\{)`)
	THINK_TAG_REGEX           = regexp.MustCompile(`(?is)\A\s*<think>(.*?)(?:</think>|\z)\s*`)
	THINK_TAG_CLOSE_REGEX     = regexp.MustCompile(`(?i)</think>`)
)

// * Prompts

//go:embed prompts/system_prompt/component/skill_execution.md
var SkillExecution string

//go:embed prompts/system_prompt/inject/summary_context.md
var SummaryContext string

//go:embed prompts/system_prompt/followup.md
var FollowupPrompt string

//go:embed prompts/system_prompt/voice.md
var VoicePrompt string

//go:embed prompts/system_prompt/inject/assign_skill.md
var AssignSkill string

//go:embed prompts/system_prompt/agent_selector.md
var AgentSelector string

//go:embed prompts/system_prompt/agent_selector_request.md
var AgentSelectorRequest string

//go:embed prompts/system_prompt/component/model_selection.md
var ModelSelection string

//go:embed prompts/system_prompt/memory/compact_exec_prompt.md
var CompactExecPrompt string

//go:embed prompts/system_prompt/memory/old_history_extract_prompt.md
var OldHistoryExtractPrompt string

//go:embed prompts/system_prompt/memory/compact_history_prompt.md
var CompactHistoryPrompt string

//go:embed prompts/system_prompt/memory/summary_prompt.md
var SummaryPrompt string

//go:embed prompts/system_prompt/system_prompt.md
var SystemPrompt string

//go:embed prompts/system_prompt/chatcompletions.md
var ChatCompletionsSystemPrompt string

//go:embed prompts/system_prompt/default_rule.md
var DefaultRule string

//go:embed prompts/system_prompt/inject/permission/always_allow.md
var PermissionAlwaysAllow string

//go:embed prompts/system_prompt/inject/permission/single_confirm.md
var PermissionSingleConfirm string

//go:embed prompts/system_prompt/subagent.md
var SubagentPrompt string

//go:embed prompts/system_prompt/component/wsl_host.md
var WSLHost string

// * Prompts > systemPrompt > Chatbot

//go:embed prompts/system_prompt/chatbot/telegram.md
var TelegramSystemPrompt string

//go:embed prompts/system_prompt/chatbot/discord.md
var DiscordSystemPrompt string

// * Prompts > systemPrompt > Inject

//go:embed prompts/system_prompt/inject/steer.md
var InjectSteer string

//go:embed prompts/system_prompt/inject/tool_summary.md
var InjectToolSummary string

//go:embed prompts/system_prompt/inject/schedule_run.md
var InjectScheduleRun string

//go:embed prompts/system_prompt/inject/shell_run.md
var InjectShellRun string

//go:embed prompts/system_prompt/inject/turn_context/work_dir.md
var InjectWorkDir string

//go:embed prompts/system_prompt/inject/turn_context/tools.md
var InjectTools string

//go:embed prompts/system_prompt/inject/turn_context/skills.md
var InjectSkills string

//go:embed prompts/system_prompt/inject/turn_context/agent_guide.md
var InjectAgentGuide string

// * Prompts > Guide

//go:embed prompts/guide/tool_generate.md
var GuideToolGenerate string

//go:embed prompts/guide/tool_error.md
var GuideToolError string

//go:embed prompts/guide/web_search.md
var GuideWebSearch string

//go:embed prompts/guide/market_analysis.md
var GuideMarketAnalysis string

//go:embed prompts/guide/targeted_read.md
var GuideTargetedRead string

//go:embed prompts/guide/ask_user.md
var GuideAskUser string

//go:embed prompts/guide/subagent_dispatch.md
var GuideSubagentDispatch string

//go:embed prompts/guide/html_render.md
var GuideHtmlRender string

//go:embed prompts/guide/write_todo.md
var GuideWriteTodo string

//go:embed prompts/guide/office.md
var GuideOffice string

// * Configs

//go:embed jsons/sensitive_path.json
var SensitivePath []byte

//go:embed jsons/exclude_list.json
var ExcludeList []byte

//go:embed jsons/read_only_command.json
var ReadOnlyCommand []byte

//go:embed jsons/tui_tools.json
var TUITools []byte

//go:embed jsons/reply_lang.json
var ReplyLang []byte

//go:embed jsons/local_compat.json
var LocalCompat []byte

//go:embed jsons/guardrail_rules.json
var GuardrailRules []byte

//go:embed jsons/refusal_messages.json
var RefusalMessages []byte

// * Official Guide

//go:embed prompts/system_prompt/official_guides/*.md
var officialGuideFS embed.FS

var OfficialGuides = loadOfficialGuides()

func loadOfficialGuides() map[string]string {
	const dir = "prompts/system_prompt/official_guides"
	entries, err := officialGuideFS.ReadDir(dir)
	if err != nil {
		return nil
	}

	guides := make(map[string]string, len(entries))
	for _, entry := range entries {
		raw, err := officialGuideFS.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			continue
		}
		guides[strings.TrimSuffix(entry.Name(), ".md")] = string(raw)
	}
	return guides
}
