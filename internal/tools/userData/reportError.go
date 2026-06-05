package userData

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	reportEndpoint = "https://report.agenvoy.com"
	uploadTimeout  = 30 * time.Second
)

func registReportError() {
	toolRegister.Regist(toolRegister.Def{
		Name:          "report_error",
		AlwaysAllow:   true,
		FireAndForget: true,
		Description:   "Collect daemon-side failures: scan daemon.log for WARN/ERROR lines in the last `h` hours and, when any are found, upload them to report.agenvoy.com (empty result uploads nothing). Returns the collected lines plus an upload-status line. Call ONLY when the current user input explicitly contains 'report error' or 'report_error' — never infer it from generic phrasing like 'check errors / what went wrong / 排錯', which route to read_log instead.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"h": map[string]any{
					"type":        "integer",
					"description": "Look-back window in hours: keep only records whose timestamp is within the last `h` hours from now. Default 1, minimum 1, maximum 72 (3 days). Lines without a parseable timestamp inherit the previous line's time (multi-line entries are kept together).",
					"default":     1,
					"minimum":     1,
					"maximum":     72,
				},
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			result, lines, err := readDeamonError(args)
			if err != nil {
				return "", err
			}
			if len(lines) == 0 {
				return result, nil
			}

			client := &http.Client{Timeout: uploadTimeout}
			if _, _, err := go_pkg_http.POST[string](ctx, client, reportEndpoint, nil, map[string]any{"report": result}, "json"); err != nil {
				return result + fmt.Sprintf("\n\n(upload to %s failed: %v)", reportEndpoint, err), nil
			}
			return result + fmt.Sprintf("\n\n(uploaded %d lines to %s)", len(lines), reportEndpoint), nil
		},
	})
}
