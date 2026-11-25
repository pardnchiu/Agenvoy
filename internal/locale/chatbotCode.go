package locale

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/pardnchiu/agenvoy/configs"
)

type ChatbotCode struct {
	VerifyPrompt    string `json:"verify_prompt"`
	PendingResolved string `json:"pending_resolved"`
	STTMissing      string `json:"stt_missing"`
}

var currentChatbotCodeDic = chatbotCodeDic()

func chatbotCodeDic() map[string]ChatbotCode {
	// NOTE: no matter err or not, always keep an en for fallback
	fallback := ChatbotCode{
		VerifyPrompt:    "Enter the 6-digit verification code printed in the daemon log.",
		PendingResolved: "Pending task already resolved in another session.",
		STTMissing:      "No speech-to-text model selected · pick one with %s first.",
	}

	var dic map[string]ChatbotCode
	if err := json.Unmarshal(configs.ConfirmChatbotNotice, &dic); err != nil || dic == nil {
		dic = map[string]ChatbotCode{}
	}
	// NOTE: ensure en is always exist for fallback
	dic["en"] = fallback
	return dic
}

func checkChatbotCode(lang string) {
	text, ok := currentChatbotCodeDic[lang]
	if !ok {
		return
	}
	if slices.Contains([]string{text.VerifyPrompt, text.PendingResolved, text.STTMissing}, "") {
		slog.Error(fmt.Sprintf("[Invalid] jsons/locale/chatbot_notice.json: %s has an empty text field, fallback to en", lang))
		delete(currentChatbotCodeDic, lang)
	}
}

func GetChatbotCode() ChatbotCode {
	mu.Lock()
	defer mu.Unlock()

	if text, ok := currentChatbotCodeDic[currentLang()]; ok {
		return text
	}
	return currentChatbotCodeDic["en"]
}
