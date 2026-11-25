// * last modified: 26/11/21
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registWriteResult() {
	toolregister.Regist(toolregister.Def{
		Name:        "write_result",
		SystemUse:   true,
		AlwaysAllow: true,
		Description: `Saves one long-form deliverable as a new timestamped .md or .html file in the output directory and returns its path; the reply then gives the key findings and the path instead of reprinting the file.
Use for "write a report", "summarize this into a document", "analyze / compare and write it up", "make it a web page", "give me html", or any answer with several sections or tables; a short answer stays in the reply.
A file at a chosen path, or changing an existing file → write_file; worked HTML page examples to copy from → html_template.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "The whole file, never a diff or an elided excerpt. Starting with <!DOCTYPE html> or <html> saves one self-contained .html page, only after reasoning_guide(topics=[html_render]), whose rules the page is checked against before writing; anything else saves a markdown .md report.",
				},
			},
			"required": []string{"content"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			newContent := strings.TrimSpace(params.Content)
			if newContent == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "content", "required")
			}

			ext := "md"
			if utils.IsHTMLDocument(newContent) {
				ext = "html"
				if problems := checkHTML(newContent); len(problems) > 0 {
					msg := "html check failed, nothing written; fix every item and call write_result again:\n- " + strings.Join(problems, "\n- ")
					return "", utilsmsg.New(utilsmsg.Invalid, "content", "%s", msg)
				}
			}

			base := filepath.Join(filesystem.OutputDir(), "report-"+time.Now().Format("20060102-150405"))
			targetPath := base + "." + ext
			// NOTE: if file exist, then append -{n} on the filename
			for i := 1; ; i++ {
				if _, err := os.Stat(targetPath); err != nil {
					break
				}
				targetPath = fmt.Sprintf("%s-%d.%s", base, i, ext)
			}
			return writeToPath(ctx, e, targetPath, newContent, "write_result")
		},
	})
}
