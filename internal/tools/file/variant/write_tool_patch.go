package variant

import (
	"context"
	"fmt"

	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeToolPatch(ctx context.Context, e *tooltypes.Executor, name, tag, oldString, newString string, replaceAll bool) (string, error) {
	if oldString == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "old_string", "required when mode=patch")
	}
	if oldString == newString {
		return "", utilsmsg.New(utilsmsg.Invalid, "new_string", "no edit needed")
	}

	target, err := toolTarget(name, tag, true)
	if err != nil {
		return "", err
	}

	change := capture(target)
	if err := patch(target, oldString, newString, replaceAll); err != nil {
		return "", err
	}
	record(ctx, e, change, "", "write_tool")

	return fmt.Sprintf("updated: %s", target), nil
}
