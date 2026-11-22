package locale

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/pardnchiu/agenvoy/configs"
)

var currentRefusal = refusalDic()

// NOTE: map[{RULE}]map[{LANG}]{TEXT}
func refusalDic() map[string]map[string]string {
	// NOTE: no matter err or not, always keep an en for fallback
	fallback := map[string]string{
		"default":      "This operation cannot be performed",
		"secret-exfil": "Sensitive information is not shown in replies; run this yourself to view it: `%s`",
	}

	var dic map[string]map[string]string
	if err := json.Unmarshal(configs.RefusalMessages, &dic); err != nil || dic == nil {
		if err != nil {
			slog.Warn("[Internal] refusal_messages: embedded JSON cannot be parsed",
				slog.String("error", err.Error()))
		}
		dic = map[string]map[string]string{}
	}

	for rule, text := range fallback {
		if dic[rule] == nil {
			dic[rule] = map[string]string{}
		}
		// NOTE: ensure en is always exist for fallback
		dic[rule]["en"] = text
	}
	return dic
}

func checkRefusal(lang string) {
	for rule, dic := range currentRefusal {
		if text, ok := dic[lang]; ok && text == "" {
			slog.Error(fmt.Sprintf("[Invalid] refusal_messages.json: %s/%s is empty, fallback to en", rule, lang))
			delete(dic, lang)
		}
	}
}

func GetRefusal(rule string) string {
	mu.Lock()
	defer mu.Unlock()

	lang := currentLang()
	dic, ok := currentRefusal[rule]
	if !ok {
		dic = currentRefusal["default"]
	}
	if text, ok := dic[lang]; ok {
		return text
	}
	// NOTE: if lang not exist, fallback to en
	return dic["en"]
}
