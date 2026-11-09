package toolSearcher

import (
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func listTools(e *tooltypes.Executor) (string, error) {
	list := make([]Tool, 0, len(e.AllTools))
	for _, tool := range e.AllTools {
		name := tool.Function.Name
		if toolregister.IsSystemUse(name) {
			continue
		}
		list = append(list, Tool{
			Name:          name,
			Description:   tool.Function.Description,
			SystemDefault: strings.HasPrefix(strings.TrimSpace(tool.Function.Description), systemDefaultMarker),
		})
	}

	raw, err := json.Marshal(list)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}
