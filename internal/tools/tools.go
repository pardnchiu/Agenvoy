package tools

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/tools/file"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
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

func moveToTrash(ctx context.Context, e *tooltypes.Executor, args []string) (string, error) {
	return file.RemoveToTrash(ctx, e, args, "rm")
}
