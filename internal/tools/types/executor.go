package toolTypes

import (
	"encoding/json"

	apiAdapter "github.com/pardnchiu/agenvoy/internal/tools/apis/adapter"
)

type Executor struct {
	WorkPath       string
	SessionID      string
	Allowed        []string // * limit to these folders to use
	AllowedCommand map[string]bool
	Tools          []Tool
	APIToolbox     *apiAdapter.Translator
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
