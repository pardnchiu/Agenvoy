package locale

import (
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	mu          sync.Mutex
	checkedLang string
)

func currentLang() string {
	lang := filesystem.CanonicalReplyLang(filesystem.GetReplyLang())
	if lang == checkedLang {
		return lang
	}

	checkedLang = lang
	if lang == "en" {
		return lang
	}

	checkConfirmDic(lang)
	checkAskUser(lang)
	checkChatbotCode(lang)
	checkChallenge(lang)
	checkRefusal(lang)
	return lang
}
