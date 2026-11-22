package locale

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/pardnchiu/agenvoy/configs"
)

type AskUser struct {
	Progress       string `json:"progress"`
	Placeholder    string `json:"placeholder"`
	Next           string `json:"next"`
	Send           string `json:"send"`
	Skip           string `json:"skip"`
	SecretRequired string `json:"secret_required"`
	DiscordSecret  string `json:"discord_secret"`
	TelegramSecret string `json:"telegram_secret"`
	HintConfirm    string `json:"hint_confirm"`
	HintToggle     string `json:"hint_toggle"`
	HintNewline    string `json:"hint_newline"`
	HintHidden     string `json:"hint_hidden"`
}

var currentAskUserDic = askUserDic()

func askUserDic() map[string]AskUser {
	// NOTE: no matter err or not, always keep an en for fallback
	fallback := AskUser{
		Progress:       "question %d/%d",
		Placeholder:    "Type your answer...",
		Next:           "Next",
		Send:           "Send",
		Skip:           "Skip",
		SecretRequired: "%s is required",
		DiscordSecret:  "Your reply is captured via a private modal and is not logged.",
		TelegramSecret: "Your reply will be deleted from chat after capture.",
		HintConfirm:    "confirm",
		HintToggle:     "toggle",
		HintNewline:    "newline",
		HintHidden:     "input hidden",
	}

	var dic map[string]AskUser
	if err := json.Unmarshal(configs.ConfirmAskUser, &dic); err != nil || dic == nil {
		dic = map[string]AskUser{}
	}
	// NOTE: ensure en is always exist for fallback
	dic["en"] = fallback
	return dic
}

func checkAskUser(lang string) {
	text, ok := currentAskUserDic[lang]
	if !ok {
		return
	}
	if slices.Contains([]string{text.Progress, text.Placeholder, text.Next, text.Send, text.Skip, text.SecretRequired, text.DiscordSecret, text.TelegramSecret, text.HintConfirm, text.HintToggle, text.HintNewline, text.HintHidden}, "") {
		slog.Error(fmt.Sprintf("[Invalid] confirm/ask_user.json: %s has an empty text field, fallback to en", lang))
		delete(currentAskUserDic, lang)
	}
}

func GetAskUser() AskUser {
	mu.Lock()
	defer mu.Unlock()

	if text, ok := currentAskUserDic[currentLang()]; ok {
		return text
	}
	// NOTE: if lang not exist, fallback to en
	return currentAskUserDic["en"]
}
