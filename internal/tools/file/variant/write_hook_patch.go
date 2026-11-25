package variant

import (
	"context"
	"fmt"

	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func writeHookPatch(ctx context.Context, e *tooltypes.Executor, name, oldString, newString string, replaceAll bool) (string, error) {
	if oldString == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "old_string", "required when mode=patch")
	}
	if oldString == newString {
		return "", utilsmsg.New(utilsmsg.Invalid, "new_string", "no edit needed")
	}

	absPath, err := hookPath(name, "")
	if err != nil {
		return "", err
	}

	change := capture(absPath)
	if err := patch(absPath, oldString, newString, replaceAll); err != nil {
		return "", err
	}
	record(ctx, e, change, "", "write_hook")

	return fmt.Sprintf("updated: %s", absPath), nil
}
