package fetchPage

import (
	"context"
	"fmt"
	"os"
	"runtime"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/locale"
	agenvoyruntime "github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func askChallenge(ctx context.Context, sessionID string) (bool, error) {
	origin := agenttypes.OriginFrom(ctx)
	if origin == "" {
		origin = agenvoyruntime.OriginOf(sessionID)
	}
	if !canShowWindow() || agenttypes.NoPromptFrom(ctx) || agenttypes.AutoModeFrom(ctx) != agenttypes.AutoModeOff || !agenvoyruntime.HasListener(ctx, origin) {
		return false, nil
	}
	text := locale.GetChallenge()
	answers, err := interactive.AskPrompt(ctx, sessionID, []agenvoyruntime.Question{{
		Question: text.Title,
		Detail:   fmt.Sprintf(text.Content, int(challengeWait.Minutes())),
		Options:  []string{text.Enable, text.Skip},
	}})
	if err != nil {
		return false, err
	}
	if len(answers) == 0 {
		return false, nil
	}
	answer, _ := answers[0].(string)
	return answer == text.Enable, nil
}

func canShowWindow() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	if utils.IsWSL() {
		return false
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func challengeDeclined(link string) error {
	addToSkippedMap(link, 403, "")
	return utilsmsg.New(utilsmsg.Upstream, link, "fetch failed [403]")
}
