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
)

func assignTurnContext(ctx context.Context, session *agenttypes.Session, workDir string, allowAll bool, skillScanner *runtime.SkillScanner, excludeSkills []string, withSkills bool, toolNames []string, toolGap bool) {
	var parts []string
	if !session.Stateless {
		parts = append(parts,
			strings.ReplaceAll(strings.TrimSpace(configs.InjectWorkDir), "{{.WorkDir}}", workDir),
			buildPermissionModeSection(allowAll),
		)
	}
	if len(toolNames) > 0 {
		parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectTools), "{{.Tools}}", strings.Join(toolNames, ", ")))
	}
	if toolGap {
		parts = append(parts, strings.TrimSpace(configs.InjectToolGap))
	}
	if withSkills {
		if list := skillListBlock(skillScanner, excludeSkills); list != "" {
			parts = append(parts, strings.ReplaceAll(strings.TrimSpace(configs.InjectSkills), "{{.Skills}}", list))
		}
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
