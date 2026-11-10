package exec

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/guide"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func assignTurnContext(ctx context.Context, session *agenttypes.Session, workDir string, skillScanner *runtime.SkillScanner, excludeSkills []string, withSkills bool, toolNames []string, toolGap bool, summary string, background []interactive.BackgroundTask) {
	var parts []string
	if !session.Stateless {
		parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectWorkDir), "{{.WorkDir}}", workDir))
	}
	if len(toolNames) > 0 {
		parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectTools), "{{.Tools}}", groupToolNames(toolNames)))
	}
	if toolGap {
		parts = append(parts, strings.TrimSpace(configs.InjectToolGap))
	}
	if withSkills {
		if list := skillListBlock(skillScanner, excludeSkills); list != "" {
			parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectSkills), "{{.Skills}}", list))
		}
	}
	if len(background) > 0 {
		lines := make([]string, 0, len(background))
		for _, one := range background {
			lines = append(lines, "- "+one.ID+": "+one.Description)
		}
		parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectBackground), "{{.Tasks}}", strings.Join(lines, "\n")))
	}
	if summary != "" {
		parts = append(parts, summary)
	}
	if !session.Stateless {
		if guideText := agentGuideSection(ctx, workDir); guideText != "" {
			parts = append(parts, guideText)
		}
	}
	if len(parts) == 0 {
		return
	}
	session.TurnContext = go_llm_router.Message{
		Role:    "user",
		Content: strings.Join(parts, "\n\n"),
	}
}

func agentGuideSection(ctx context.Context, workDir string) string {
	if !guide.IsEnabledCtx(ctx) {
		return ""
	}
	for _, name := range guide.Files {
		path := filepath.Join(workDir, name)
		if !go_pkg_filesystem_reader.IsFile(path) {
			continue
		}

		content, err := go_pkg_filesystem.ReadText(path)
		if err != nil {
			slog.Debug("agent guide ReadText",
				slog.String("path", path),
				slog.String("error", err.Error()))
			continue
		}
		if content = strings.TrimSpace(content); content == "" {
			continue
		}
		return strings.NewReplacer(
			"{{.Path}}", path,
			"{{.Content}}", content,
		).Replace(strings.TrimSpace(configs.InjectAgentGuide))
	}
	return ""
}

func groupToolNames(names []string) string {
	var order []string
	serverNames := map[string][]string{}
	for _, name := range names {
		rest, ok := strings.CutPrefix(name, "mcp__")
		server, tool, found := strings.Cut(rest, "__")
		if !ok || !found || server == "" || tool == "" {
			order = append(order, name)
			continue
		}
		prefix := "mcp__" + server + "__"
		if _, seen := serverNames[prefix]; !seen {
			order = append(order, prefix)
		}
		serverNames[prefix] = append(serverNames[prefix], tool)
	}
	parts := make([]string, 0, len(order))
	for _, one := range order {
		if tools, ok := serverNames[one]; ok {
			parts = append(parts, one+"{"+strings.Join(tools, ", ")+"}")
			continue
		}
		parts = append(parts, one)
	}
	return strings.Join(parts, ", ")
}
