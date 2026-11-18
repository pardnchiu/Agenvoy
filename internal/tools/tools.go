package tools

import (
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
)

var (
	TUIOnlyTools  []string
	TUIOnlySkills []string
)

func init() {
	var data struct {
		Tools  []string `json:"tools"`
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal(configs.TUITools, &data); err != nil {
		return
	}
	TUIOnlyTools = data.Tools
	TUIOnlySkills = data.Skills
}
