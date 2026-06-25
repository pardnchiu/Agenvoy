package configBot

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	DefaultModel     = "auto"
	DefaultReasoning = "medium"
)

var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	fieldRegex       = regexp.MustCompile(`(?m)^(\w+):\s*(.+)$`)
)

type Bot struct {
	Name      string
	Model     string
	Reasoning string
	Body      string
}

func read(sessionID string) Bot {
	if sessionID == "" {
		return Bot{}
	}
	data, err := go_pkg_filesystem.ReadText(filesystem.BotPath(sessionID))
	if err != nil {
		return Bot{}
	}
	m := frontmatterRegex.FindStringSubmatch(data)
	if len(m) < 3 {
		return Bot{Body: strings.TrimSpace(data)}
	}

	bot := Bot{Body: strings.TrimSpace(m[2])}
	for _, fm := range fieldRegex.FindAllStringSubmatch(m[1], -1) {
		switch fm[1] {
		case "name":
			bot.Name = strings.TrimSpace(fm[2])
		case "model":
			bot.Model = strings.TrimSpace(fm[2])
		case "reasoning":
			bot.Reasoning = strings.TrimSpace(fm[2])
		}
	}
	return bot
}

func writeBotFile(sessionID string, bot Bot) error {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %s\n", bot.Name)
	if bot.Model != "" {
		fmt.Fprintf(&sb, "model: %s\n", bot.Model)
	}
	if bot.Reasoning != "" {
		fmt.Fprintf(&sb, "reasoning: %s\n", bot.Reasoning)
	}
	sb.WriteString("---\n")
	sb.WriteString(bot.Body)

	if err := go_pkg_filesystem.WriteFile(filesystem.BotPath(sessionID), sb.String(), 0644); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
	}
	return nil
}

func Get(sessionID string) (name, body string) {
	bot := read(sessionID)
	return bot.Name, bot.Body
}

func GetModel(sessionID string) (model, reasoning string) {
	bot := read(sessionID)
	model = bot.Model
	reasoning = bot.Reasoning
	if model == "" {
		model = DefaultModel
	}
	if reasoning == "" {
		reasoning = DefaultReasoning
	}
	return model, reasoning
}

func SetModel(sessionID, model, reasoning string) {
	if sessionID == "" {
		return
	}
	bot := read(sessionID)
	if model != "" {
		bot.Model = model
	}
	if reasoning != "" {
		bot.Reasoning = reasoning
	}
	writeBotFile(sessionID, bot)
}

func FormatName(raw string) string {
	var sb strings.Builder
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func ReplaceDefault(sessionID, name string) {
	if sessionID == "" || name == "" {
		return
	}
	bot := read(sessionID)
	if bot.Name != "" && !strings.HasPrefix(bot.Name, "tg-") && !strings.HasPrefix(bot.Name, "dc-") {
		return
	}
	bot.Name = name
	writeBotFile(sessionID, bot)
}

func Save(sessionID, name, body string, force bool) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID is required")
	}
	if name == "" {
		name = sessionID
	}
	if body == "" {
		body = configs.DefaultSessionPrompt
	}

	path := filesystem.BotPath(sessionID)
	if !force && go_pkg_filesystem_reader.Exists(path) {
		return nil
	}

	return writeBotFile(sessionID, Bot{Name: name, Body: body})
}
