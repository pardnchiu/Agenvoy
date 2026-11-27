package locale

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var webUIDic = func() map[string]map[string]string {
	entries, err := configs.I18nFS.ReadDir("jsons/locale/i18n")
	if err != nil {
		panic(utilsmsg.Wrap(utilsmsg.Internal, "configs.I18nFS: ReadDir", "jsons/locale/i18n", err))
	}
	dic := make(map[string]map[string]string, len(entries))
	for _, entry := range entries {
		name := "jsons/locale/i18n/" + entry.Name()
		raw, err := configs.I18nFS.ReadFile(name)
		if err != nil {
			panic(utilsmsg.Wrap(utilsmsg.Internal, "configs.I18nFS: ReadFile", name, err))
		}
		var one map[string]string
		if err := json.Unmarshal(raw, &one); err != nil {
			panic(utilsmsg.Wrap(utilsmsg.Internal, "json: Unmarshal", name, err))
		}
		dic[filesystem.CanonicalReplyLang(strings.TrimSuffix(entry.Name(), ".json"))] = one
	}
	if dic["en"] == nil {
		panic(utilsmsg.New(utilsmsg.Internal, "jsons/locale/i18n", "missing en.json"))
	}
	return dic
}()

func WebUI(browser string) (string, map[string]string) {
	lang := filesystem.CanonicalReplyLang(filesystem.GetReplyLang())
	if lang == filesystem.ReplyLangAuto {
		lang = browserLang(browser)
	}
	out := maps.Clone(webUIDic["en"])
	one, ok := webUIDic[lang]
	if !ok {
		return "en", out
	}
	for key, text := range one {
		if text != "" {
			out[key] = text
		}
	}
	return lang, out
}

func browserLang(browser string) string {
	code := strings.ToLower(strings.TrimSpace(browser))
	switch {
	case code == "":
		return "en"
	case strings.HasPrefix(code, "zh") && (strings.Contains(code, "-hk") || strings.Contains(code, "-mo")):
		return "zh-HK"
	case strings.HasPrefix(code, "zh-hant"), strings.HasPrefix(code, "zh-tw"):
		return "zh-TW"
	case strings.HasPrefix(code, "zh"):
		return "zh-CN"
	}
	if lang := filesystem.CanonicalReplyLang(code); lang != code {
		return lang
	}
	primary, _, _ := strings.Cut(code, "-")
	return filesystem.CanonicalReplyLang(primary)
}
