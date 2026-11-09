package configBot

import (
	"context"
	"log/slog"
	"strings"
	"unicode"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	DefaultModel     = historystore.DefaultModel
	DefaultReasoning = historystore.DefaultReasoning
)

func ReasoningLevels() []string {
	out := make([]string, 0, int(go_llm_router.ReasoningMax)+2)
	out = append(out, configs.REASONING_AUTO)
	for r := go_llm_router.ReasoningNone; r <= go_llm_router.ReasoningMax; r++ {
		out = append(out, r.String())
	}
	return out
}

func read(sessionID string) (historystore.SessionRow, bool) {
	if sessionID == "" {
		return historystore.SessionRow{}, false
	}

	row, ok, err := historystore.ReadSession(context.Background(), sessionID)
	if err != nil {
		slog.Warn("historystore.ReadSession",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return historystore.SessionRow{}, false
	}
	row.SessionID = sessionID
	return row, ok
}

func write(row historystore.SessionRow) error {
	if err := historystore.WriteSession(context.Background(), row); err != nil {
		return utilsmsg.Trace("historystore.WriteSession", "", err)
	}
	return nil
}

func Get(sessionID string) (name, body string) {
	row, _ := read(sessionID)
	return row.Name, row.Role
}

func GetPersona(sessionID string) (selfID, name, body string) {
	row, _ := read(sessionID)
	return row.SelfID, row.Name, row.Role
}

func SavePersona(sessionID, selfID, name, body string) error {
	if sessionID == "" {
		return utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}
	if err := historystore.ValidSelfID(selfID); err != nil {
		return err
	}
	if body == "" {
		body = configs.DefaultRule
	}

	row, _ := read(sessionID)
	row.SessionID = sessionID
	row.SelfID = selfID
	row.Name = name
	row.Role = body
	return write(row)
}

func GetModel(sessionID string) (model, reasoning string) {
	row, _ := read(sessionID)
	model = config.NormalizeModel(row.Model)
	reasoning = row.Reasoning
	if model == "" {
		model = DefaultModel
	}
	if reasoning == "" {
		reasoning = DefaultReasoning
	}
	return model, reasoning
}

func SetModel(sessionID, model, reasoning string) {
	row, _ := read(sessionID)
	if row.SessionID == "" {
		return
	}
	if model != "" {
		row.Model = config.NormalizeModel(model)
	}
	if reasoning != "" {
		row.Reasoning = reasoning
	}
	if err := write(row); err != nil {
		slog.Warn("configBot SetModel",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func ResetModel(name string) {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return
	}
	for _, dir := range dirs {
		if strings.HasPrefix(dir.Name, ".") {
			continue
		}
		if model, _ := GetModel(dir.Name); model == name {
			SetModel(dir.Name, DefaultModel, "")
		}
	}
}

func SetChannel(sessionID string, chatID, guildID, channelID, userID string) error {
	row, _ := read(sessionID)
	if row.SessionID == "" {
		return utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}
	row.ChatID = chatID
	row.GuildID = guildID
	row.ChannelID = channelID
	row.UserID = userID
	return write(row)
}

func GetChannel(sessionID string) (chatID, guildID, channelID, userID string) {
	row, _ := read(sessionID)
	return row.ChatID, row.GuildID, row.ChannelID, row.UserID
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

func autoNamed(row historystore.SessionRow) bool {
	return row.Name == "" ||
		row.Name == row.SessionID ||
		row.Name == utils.ShortenSessionID(row.SessionID) ||
		strings.HasPrefix(row.Name, "tg-") ||
		strings.HasPrefix(row.Name, "dc-") ||
		strings.HasPrefix(row.Name, "ln-")
}

func ReplaceDefault(sessionID, name string) {
	if name == "" {
		return
	}
	row, _ := read(sessionID)
	if row.SessionID == "" {
		return
	}
	if !autoNamed(row) {
		return
	}

	row.Name = name
	if err := write(row); err != nil {
		slog.Warn("configBot ReplaceDefault",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func NeedTitle(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	row, _ := read(sessionID)
	return autoNamed(row)
}

func SetTitle(sessionID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" || !NeedTitle(sessionID) {
		return nil
	}

	row, _ := read(sessionID)
	if row.SessionID == "" {
		return utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}
	row.Name = title
	return write(row)
}

func Save(sessionID, name, body string, force bool) error {
	if sessionID == "" {
		return utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}
	if body == "" {
		body = configs.DefaultRule
	}

	row, exists := read(sessionID)
	if !force && exists {
		return nil
	}

	row.SessionID = sessionID
	row.Name = name
	row.Role = body
	if err := write(row); err != nil {
		return err
	}

	dir := filesystem.SessionDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
	}
	return nil
}
