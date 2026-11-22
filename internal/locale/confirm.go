package locale

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	maxConfirmLabelBytes = 64
)

type Confirm struct {
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
	Yes               string `json:"yes"`
	Remember          string `json:"remember"`
	AllowTurn         string `json:"allow_turn"`
	Skip              string `json:"skip"`
	Abort             string `json:"abort"`
	Cancel            string `json:"cancel"`
}

var currentConfirmDic = confirmDic()

func confirmDic() map[string]Confirm {
	// NOTE: no matter err or not, always keep an en for fallback
	fallback := Confirm{
		Title:             "Run %s?",
		TitleRestricted:   "Run %s outside the allow list?",
		PasswordRequired:  "system password required",
		PasswordCached:    "sudo credentials still valid — no password needed",
		ChatRestricted:    "chat cannot collect it: approve it from the TUI or Web UI on this machine",
		Password:          "System password",
		PasswordEmpty:     "type your system password first",
		PasswordRejected:  "password rejected",
		Reason:            "Why not? Tell it what to do instead (enter to skip)",
		Send:              "send",
		SkipHeader:        "Skipped. Add a reason for the agent?",
		SkipWithoutReason: "Skip without reason",
		SkipWriteReason:   "Write a reason",
		SkipReasonHeader:  "Reason for skipping",
		Yes:               "Yes",
		Remember:          "Yes, don't ask again",
		AllowTurn:         "Yes, allow this turn",
		Skip:              "No, skip this",
		Abort:             "Abort, keep for resume",
		Cancel:            "Cancel, delete task",
	}

	var dic map[string]Confirm
	if err := json.Unmarshal(configs.ConfirmTool, &dic); err != nil || dic == nil {
		dic = map[string]Confirm{}
	}
	// NOTE: ensure en is always exist for fallback
	dic["en"] = fallback
	return dic
}

func checkConfirmDic(lang string) {
	text, ok := currentConfirmDic[lang]
	if !ok {
		return
	}

	buttons := append(text.Options(), text.SkipWithoutReason, text.SkipWriteReason)
	for _, label := range buttons {
		if label == "" || len(label) > maxConfirmLabelBytes {
			slog.Error(fmt.Sprintf("[Invalid] confirm/tool_confirm.json: %s label %q must be 1-%d bytes, fallback to en", lang, label, maxConfirmLabelBytes))
			delete(currentConfirmDic, lang)
			return
		}
	}

	if slices.Contains([]string{text.Title, text.TitleRestricted, text.PasswordRequired, text.PasswordCached, text.ChatRestricted, text.Password, text.PasswordEmpty, text.PasswordRejected, text.Reason, text.Send, text.SkipHeader, text.SkipReasonHeader}, "") {
		slog.Error(fmt.Sprintf("[Invalid] confirm/tool_confirm.json: %s has an empty text field, fallback to en", lang))
		delete(currentConfirmDic, lang)
	}
}

func (c Confirm) Options() []string {
	return []string{c.Yes, c.Remember, c.AllowTurn, c.Skip, c.Abort, c.Cancel}
}

func GetConfirm() Confirm {
	mu.Lock()
	defer mu.Unlock()

	if one, ok := currentConfirmDic[currentLang()]; ok {
		return one
	}
	// NOTE: if lang not exist, fallback to en
	return currentConfirmDic["en"]
}

func AllConfirm() []Confirm {
	mu.Lock()
	defer mu.Unlock()

	list := make([]Confirm, 0, len(currentConfirmDic))
	for _, e := range currentConfirmDic {
		if slices.Contains(append(e.Options(), e.SkipWithoutReason, e.SkipWriteReason), "") {
			continue
		}
		list = append(list, e)
	}
	return list
}
