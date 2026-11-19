package filesystem

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	maxConfirmLabelBytes = 56
	tuiConfirmSeparator  = "  "
)

type confirmEntry struct {
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
		for _, label := range one.labels(one.Separator).List() {
			if label == "" || len(label) > maxConfirmLabelBytes {
				panic(fmt.Sprintf("[Invalid] confirm/tool_confirm.json: %s label %q must be 1-%d bytes", lang, label, maxConfirmLabelBytes))
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

func AllConfirmLabels() []ConfirmLabels {
	list := make([]ConfirmLabels, 0, len(langConfirmEntry))
	for _, one := range langConfirmEntry {
		list = append(list, one.labels(one.Separator))
	}
	return list
}
