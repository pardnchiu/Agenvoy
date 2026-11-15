package file

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
)

func checkFileRelevance(ctx context.Context, sessionID, objective, path, content string) string {
	if !decision.Withhold(ctx, sessionID, "file:"+path, objective, content, slog.String("path", path)) {
		return content
	}
	return fmt.Sprintf("File withheld: %s does not bear on the current request, so its content was not loaded. Read it again only if it is needed after all; a second read returns the content.", path)
}
