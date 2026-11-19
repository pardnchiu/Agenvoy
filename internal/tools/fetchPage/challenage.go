package fetchPage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	agenvoyruntime "github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type challengeText struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Enable  string `json:"enable"`
	Skip    string `json:"skip"`
}

var challengeTexts = loadChallengeTexts()

func loadChallengeTexts() map[string]challengeText {
	var dic map[string]challengeText
	if err := json.Unmarshal(configs.ConfirmFetchChallenge, &dic); err != nil {
		panic(fmt.Sprintf("[Internal] confirm/fetch_challenge.json: %v", err))
	}
	if _, ok := dic["en"]; !ok {
		panic(`[Internal] confirm/fetch_challenge.json: missing "en"`)
	}
	return dic
}

func getChallengeText() challengeText {
	if text, ok := challengeTexts[filesystem.CanonicalReplyLang(filesystem.ConfigReplyLang)]; ok {
		return text
	}
	return challengeTexts["en"]
}

func askChallenge(ctx context.Context, sessionID string) (bool, error) {
	origin := agenttypes.OriginFrom(ctx)
	if origin == "" {
		origin = agenvoyruntime.OriginOf(sessionID)
	}
	if !canShowWindow() || agenttypes.NoPromptFrom(ctx) || agenttypes.AutoModeFrom(ctx) != agenttypes.AutoModeOff || !agenvoyruntime.HasListener(ctx, origin) {
		return false, nil
	}
	text := getChallengeText()
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
