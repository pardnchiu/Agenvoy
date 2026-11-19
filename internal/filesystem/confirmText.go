package filesystem

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	maxConfirmLabelBytes = 64
	tuiConfirmSeparator  = "  "
)

type ConfirmText struct {
	Title             string `json:"title"`
	TitleRestricted   string `json:"title_restricted"`
	PasswordRequired  string `json:"password_required"`
	PasswordCached    string `json:"password_cached"`
	ChatRestricted    string `json:"chat_restricted"`
	Password          string `json:"password"`
	PasswordEmpty     string `json:"password_empty"`
	PasswordRejected  string `json:"password_rejected"`
	Reason            string `json:"reason"`
	Send              string `json:"send"`
	SkipHeader        string `json:"skip_header"`
	SkipWithoutReason string `json:"skip_without_reason"`
	SkipWriteReason   string `json:"skip_write_reason"`
	SkipReasonHeader  string `json:"skip_reason_header"`
}

type confirmEntry struct {
	ConfirmText
	Separator string   `json:"separator"`
	Yes       []string `json:"yes"`
	Remember  []string `json:"remember"`
	AllowTurn []string `json:"allow_turn"`
	Skip      []string `json:"skip"`
	Abort     []string `json:"abort"`
	Cancel    []string `json:"cancel"`
}

type ConfirmLabels struct {
	Yes       string `json:"yes"`
	Remember  string `json:"remember"`
	AllowTurn string `json:"allow_turn"`
	Skip      string `json:"skip"`
	Abort     string `json:"abort"`
	Cancel    string `json:"cancel"`
}

var langConfirmEntry = loadConfirmEntries()

func loadConfirmEntries() map[string]confirmEntry {
	var dic map[string]confirmEntry
	if err := json.Unmarshal(configs.ConfirmTool, &dic); err != nil {
		panic(fmt.Sprintf("[Internal] confirm/tool_confirm.json: %v", err))
	}
	if _, ok := dic["en"]; !ok {
		panic(`[Internal] confirm/tool_confirm.json: missing "en"`)
	}
	for lang, one := range dic {
		buttons := append(one.labels(one.Separator).List(), one.SkipWithoutReason, one.SkipWriteReason)
		for _, label := range buttons {
			if label == "" || len(label) > maxConfirmLabelBytes {
				panic(fmt.Sprintf("[Invalid] confirm/tool_confirm.json: %s label %q must be 1-%d bytes", lang, label, maxConfirmLabelBytes))
			}
		}
		text := one.ConfirmText
		for _, value := range []string{text.Title, text.TitleRestricted, text.PasswordRequired, text.PasswordCached, text.ChatRestricted, text.Password, text.PasswordEmpty, text.PasswordRejected, text.Reason, text.Send, text.SkipHeader, text.SkipReasonHeader} {
			if value == "" {
				panic(fmt.Sprintf("[Invalid] confirm/tool_confirm.json: %s has an empty text field", lang))
			}
		}
	}
	return dic
}

func (e confirmEntry) labels(sep string) ConfirmLabels {
	return ConfirmLabels{
		Yes:       strings.Join(e.Yes, sep),
		Remember:  strings.Join(e.Remember, sep),
		AllowTurn: strings.Join(e.AllowTurn, sep),
		Skip:      strings.Join(e.Skip, sep),
		Abort:     strings.Join(e.Abort, sep),
		Cancel:    strings.Join(e.Cancel, sep),
	}
}

func (c ConfirmLabels) List() []string {
	return []string{c.Yes, c.Remember, c.AllowTurn, c.Skip, c.Abort, c.Cancel}
}

func currentConfirmEntry() confirmEntry {
	if one, ok := langConfirmEntry[CanonicalReplyLang(ConfigReplyLang)]; ok {
		return one
	}
	return langConfirmEntry["en"]
}

func GetConfirmLabels() ConfirmLabels {
	one := currentConfirmEntry()
	return one.labels(one.Separator)
}

func GetTUIConfirmLabels() ConfirmLabels {
	return currentConfirmEntry().labels(tuiConfirmSeparator)
}

func GetConfirmText() ConfirmText {
	return currentConfirmEntry().ConfirmText
}

func AllConfirmText() []ConfirmText {
	list := make([]ConfirmText, 0, len(langConfirmEntry))
	for _, one := range langConfirmEntry {
		list = append(list, one.ConfirmText)
	}
	return list
}

func AllConfirmLabels() []ConfirmLabels {
	list := make([]ConfirmLabels, 0, len(langConfirmEntry))
	for _, one := range langConfirmEntry {
		list = append(list, one.labels(one.Separator))
	}
	return list
}

type AskUserText struct {
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

var langAskUserText = loadAskUserText()

func loadAskUserText() map[string]AskUserText {
	var dic map[string]AskUserText
	if err := json.Unmarshal(configs.ConfirmAskUser, &dic); err != nil {
		panic(fmt.Sprintf("[Internal] confirm/ask_user.json: %v", err))
	}
	if _, ok := dic["en"]; !ok {
		panic(`[Internal] confirm/ask_user.json: missing "en"`)
	}
	for lang, one := range dic {
		for _, value := range []string{one.Progress, one.Placeholder, one.Next, one.Send, one.Skip, one.SecretRequired, one.DiscordSecret, one.TelegramSecret, one.HintConfirm, one.HintToggle, one.HintNewline, one.HintHidden} {
			if value == "" {
				panic(fmt.Sprintf("[Invalid] confirm/ask_user.json: %s has an empty text field", lang))
			}
		}
	}
	return dic
}

func GetAskUserText() AskUserText {
	if one, ok := langAskUserText[CanonicalReplyLang(ConfigReplyLang)]; ok {
		return one
	}
	return langAskUserText["en"]
}

type ChatbotNotice struct {
	VerifyPrompt    string `json:"verify_prompt"`
	PendingResolved string `json:"pending_resolved"`
	STTMissing      string `json:"stt_missing"`
}

var langChatbotNotice = loadChatbotNotice()

func loadChatbotNotice() map[string]ChatbotNotice {
	var dic map[string]ChatbotNotice
	if err := json.Unmarshal(configs.ConfirmChatbotNotice, &dic); err != nil {
		panic(fmt.Sprintf("[Internal] confirm/chatbot_notice.json: %v", err))
	}
	if _, ok := dic["en"]; !ok {
		panic(`[Internal] confirm/chatbot_notice.json: missing "en"`)
	}
	for lang, one := range dic {
		for _, value := range []string{one.VerifyPrompt, one.PendingResolved, one.STTMissing} {
			if value == "" {
				panic(fmt.Sprintf("[Invalid] confirm/chatbot_notice.json: %s has an empty text field", lang))
			}
		}
	}
	return dic
}

func GetChatbotNotice() ChatbotNotice {
	if one, ok := langChatbotNotice[CanonicalReplyLang(ConfigReplyLang)]; ok {
		return one
	}
	return langChatbotNotice["en"]
}
