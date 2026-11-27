package tools

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/tools/calculate"
	"github.com/pardnchiu/agenvoy/internal/tools/command"
	toolcron "github.com/pardnchiu/agenvoy/internal/tools/cron"
	"github.com/pardnchiu/agenvoy/internal/tools/fetchPage"
	"github.com/pardnchiu/agenvoy/internal/tools/file"
	_ "github.com/pardnchiu/agenvoy/internal/tools/history/action"
	_ "github.com/pardnchiu/agenvoy/internal/tools/history/error"
	_ "github.com/pardnchiu/agenvoy/internal/tools/history/file"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/searchWeb"
	"github.com/pardnchiu/agenvoy/internal/tools/toolAdapter"
	"github.com/pardnchiu/agenvoy/internal/tools/toolSkill"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func init() {
	registReasoningGuide()

	toolAdapter.Register()
	searchWeb.Register()
	fetchPage.Register()
	file.Register()
	toolcron.Register()
	toolSkill.Register()
	interactive.Register()
	command.Register()

	registOpenFile()
	registHTMLTemplate()
	registPublicAPIList()
	registImage()
	registAudio()

	toolregister.RegistGroup(configs.SCRIPT_TOOL_PREFIX, func(ctx context.Context, e *tooltypes.Executor, name string, args json.RawMessage) (string, error) {
		if e.ScriptToolbox == nil || !e.ScriptToolbox.IsExist(name) {
			return "", utilsmsg.New(utilsmsg.NotFound, "tool", "not exist: %s", name)
		}
		return e.ScriptToolbox.Execute(ctx, name, args, e.WorkDir)
	})

	toolregister.RegistGroup("ext_", func(ctx context.Context, e *tooltypes.Executor, name string, args json.RawMessage) (string, error) {
		if e.ExtScriptToolbox != nil && e.ExtScriptToolbox.IsExist(name) {
			return e.ExtScriptToolbox.Execute(ctx, name, args, e.WorkDir)
		}
		return "", utilsmsg.New(utilsmsg.NotFound, "tool", "not exist: %s", name)
	})

	calculate.Register()
}
