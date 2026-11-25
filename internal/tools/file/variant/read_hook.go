package variant

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registReadHook() {
	toolregister.Regist(toolregister.Def{
		Name:        "read_hook",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Returns one hook state's demo Python script, showing every field that state receives on stdin, plus the Python / JavaScript hook currently installed for that state (.py over .js), or (none).
Use before writing or changing a hook, to see its fields and what it already does.
Writing the hook → write_hook.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"enum":        agentsexechook.Names(),
					"description": "The hook state to read.",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(_ context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			name := strings.TrimSpace(params.Name)
			if !slices.Contains(agentsexechook.Names(), name) {
				return "", utilsmsg.New(utilsmsg.Invalid, "name", "unknown hook %q; available: %s", name, strings.Join(agentsexechook.Names(), ", "))
			}
			demoPath := filepath.Join(filesystem.HooksDir, "demo", name+".py")
			if !go_pkg_filesystem_reader.IsFile(demoPath) {
				return "", utilsmsg.New(utilsmsg.NotFound, "hook demo", "%s", demoPath)
			}
			demo, err := go_pkg_filesystem.ReadText(demoPath)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: ReadText", demoPath, err)
			}

			current := "(none)"
			for _, ext := range []string{".py", ".js"} {
				path := filepath.Join(filesystem.HooksDir, name+ext)
				if !go_pkg_filesystem_reader.IsFile(path) {
					continue
				}
				text, err := go_pkg_filesystem.ReadText(path)
				if err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: ReadText", path, err)
				}
				current = path + "\n\n" + text
				break
			}

			return "## demo\n\n" + demo + "\n## current\n\n" + current, nil
		},
	})
}
