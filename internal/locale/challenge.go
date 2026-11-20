package locale

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/pardnchiu/agenvoy/configs"
)

type Challenge struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Enable  string `json:"enable"`
	Skip    string `json:"skip"`
}

var currentChallenageDic = challengeDic()

func challengeDic() map[string]Challenge {
	// NOTE: no matter err or not, always keep an en for fallback
	fallback := Challenge{
		Title:   "The target site requires human verification. Enable manual verification?",
		Content: "1. You need to be at this computer: a browser window will open here for you to complete the check\n2. Times out after %d minutes; the window then closes and this page is skipped",
		Enable:  "Enable manual verification",
		Skip:    "Skip",
	}

	var dic map[string]Challenge
	if err := json.Unmarshal(configs.ConfirmFetchChallenge, &dic); err != nil || dic == nil {
		dic = map[string]Challenge{}
	}
	// NOTE: ensure en is always exist for fallback
	dic["en"] = fallback
	return dic
}

func checkChallenge(lang string) {
	text, ok := currentChallenageDic[lang]
	if !ok {
		return
	}
	if text.Title == "" || text.Content == "" || text.Enable == "" || text.Skip == "" {
		slog.Error(fmt.Sprintf("[Invalid] confirm/fetch_challenge.json: %s has an empty text field, fallback to en", lang))
		delete(currentChallenageDic, lang)
	}
}

func GetChallenge() Challenge {
	mu.Lock()
	defer mu.Unlock()

	if text, ok := currentChallenageDic[currentLang()]; ok {
		return text
	}
	// NOTE: if lang not exist, fallback to en
	return currentChallenageDic["en"]
}
