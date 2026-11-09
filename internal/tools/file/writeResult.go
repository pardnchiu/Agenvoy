package file

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registWriteResult() {
	toolregister.Regist(toolregister.Def{
		Name:        "write_result",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `Saves one long-form deliverable as a new .md or .html file in the output directory and returns its path.
Use for 報告 / 整理 / 分析 / 比較 / 做成網頁 / html, or any answer with several sections or tables; the reply then gives its key findings and the path instead of reprinting it, and a short answer stays in the reply. html only after reasoning_guide(topics=[html_render]).
Any other file, or changing an existing one → edit_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{
					"type":        "string",
					"enum":        []string{"md", "html"},
					"default":     "md",
					"description": "md for a markdown report, html for one self-contained page.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The whole file in that format, never a diff or an elided excerpt.",
				},
			},
			"required": []string{"content"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Format  string `json:"format"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			switch params.Format {
			case "":
				params.Format = "md"
			case "md", "html":
			default:
				return "", utilsmsg.New(utilsmsg.Invalid, "format", "unknown %q; available: md, html", params.Format)
			}

			if err := rejectElided(params.Content, nil); err != nil {
				return "", err
			}

			if params.Format == "html" {
				if problems, quickUI := checkHTML(params.Content); len(problems) > 0 {
					msg := "html check failed, nothing written; fix every item and call write_result again:\n- " + strings.Join(problems, "\n- ")
					if !quickUI {
						msg += "\n\nThe page was not built per html_render; rebuild it against these rules:\n\n## html_render\n\n" + strings.TrimSpace(configs.GuideHtmlRender)
					}
					return "", errors.New(msg)
				}
			}

			return writeFileContent(ctx, e, resultPath(params.Format), params.Content, "write_result")
		},
	})
}

func resultPath(ext string) string {
	return filepath.Join(filesystem.OutputDir(), "report-"+time.Now().Format("20060102-150405")+"."+ext)
}
