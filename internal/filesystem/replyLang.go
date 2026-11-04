package filesystem

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

type replyLangEntry struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Note  string `json:"note"`
}

type ReplyLangOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

const replyLangAutoLabel = "follow the language of each message"

var (
	replyLangList = loadReplyLang()
	codeReplyLang = indexReplyLang()
)

func loadReplyLang() []replyLangEntry {
	var out []replyLangEntry
	if err := json.Unmarshal(configs.ReplyLang, &out); err != nil {
		slog.Warn("embedded reply_lang",
			slog.String("error", err.Error()))
	}
	return out
}

func indexReplyLang() map[string]replyLangEntry {
	dic := make(map[string]replyLangEntry, len(replyLangList))
	for _, one := range replyLangList {
		dic[normalizeReplyLang(one.Code)] = one
	}
	return dic
}

func normalizeReplyLang(code string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "_", "-"))
}

func ReplyLangOptions() []ReplyLangOption {
	out := make([]ReplyLangOption, 0, len(replyLangList)+1)
	out = append(out, ReplyLangOption{Code: ReplyLangAuto, Label: replyLangAutoLabel})
	for _, one := range replyLangList {
		label := one.Label
		if label == "" {
			label = one.Name
		}
		out = append(out, ReplyLangOption{Code: one.Code, Label: label})
	}
	return out
}

func CanonicalReplyLang(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || strings.EqualFold(code, ReplyLangAuto) {
		return ReplyLangAuto
	}
	if one, ok := codeReplyLang[normalizeReplyLang(code)]; ok {
		return one.Code
	}
	return code
}

func ReplyLangName(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || strings.EqualFold(code, ReplyLangAuto) {
		return ""
	}
	if one, ok := codeReplyLang[normalizeReplyLang(code)]; ok {
		return one.Name
	}
	return code
}

func ReplyLangDirective() string {
	code := strings.TrimSpace(ConfigReplyLang)
	if code == "" || strings.EqualFold(code, ReplyLangAuto) {
		return ""
	}

	one, ok := codeReplyLang[normalizeReplyLang(code)]
	if !ok {
		one = replyLangEntry{Name: code}
	}
	return one.Name + " by default." + one.Note + " The operator set this default, so the language the user writes in does not change it. When the current request asks for another language (\"英文\", \"in English\", \"日本語で\") or a skill, template or tool in use fixes the output's language (a bilingual commit message, an English-only field), follow that for the output it covers and keep the rest in " + one.Name + ". Otherwise no mixing."
}

const (
	replyLangAutoOpen  = "<reply-lang-auto>"
	replyLangAutoClose = "</reply-lang-auto>"
)

func ApplyReplyLang(template string) string {
	keep := ReplyLangDirective() == ""
	for {
		start := strings.Index(template, replyLangAutoOpen)
		if start < 0 {
			return template
		}
		end := strings.Index(template[start:], replyLangAutoClose)
		if end < 0 {
			return template
		}
		end += start

		inner := template[start+len(replyLangAutoOpen) : end]
		if !keep {
			inner = ""
		}
		template = template[:start] + inner + template[end+len(replyLangAutoClose):]
	}
}

var langRefusal = getLangRefusal()

func getLangRefusal() map[string]string {
	var dic map[string]string
	if err := json.Unmarshal(configs.RefusalMessages, &dic); err != nil {
		slog.Warn("embedded poison_refusal",
			slog.String("error", err.Error()))
	}
	return dic
}

func RefusalMessage() string {
	if text, ok := langRefusal[CanonicalReplyLang(ConfigReplyLang)]; ok {
		return text
	}
	return langRefusal["en"]
}
