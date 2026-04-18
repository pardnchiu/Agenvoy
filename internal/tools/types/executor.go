package toolTypes

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/internal/skill"
	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
)

type ScriptToolExecutor interface {
	IsExist(name string) bool
	Execute(ctx context.Context, name string, args json.RawMessage, workDir string) (string, error)
	GetTools() []map[string]any
}

type Executor struct {
	WorkDir        string
	SessionID      string
	Allowed        []string // * limit to these folders to use
	AllowedCommand map[string]bool
	Tools          []Tool
	AllTools       []Tool
	StubTools      map[string]bool
	ExcludeTools   map[string]bool
	APIToolbox     *apiAdapter.Translator
	ScriptToolbox  ScriptToolExecutor

	ActiveSkill  *skill.Skill
	SkillScanner *skill.SkillScanner
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
