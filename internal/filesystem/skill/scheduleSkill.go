package skill

import (
	"context"
	"log/slog"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func ReadSchedule(name string) (string, error) {
	if err := filesystem.CheckDirName("schedule skill name", name); err != nil {
		return "", err
	}
	path := filesystem.ScheduleSkillPath(name)
	if !go_pkg_filesystem_reader.Exists(path) {
		return "", utilsmsg.New(utilsmsg.NotFound, "schedule skill", "%s", name)
	}

	result, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadText", path, err)
	}
	return result, nil
}

func WriteSchedule(name, body string) error {
	if bodyRegex.MatchString(strings.TrimLeft(body, " \t\r\n")) {
		return writeScheduleRaw(name, strings.TrimSpace(strings.TrimLeft(body, " \t\r\n"))+"\n")
	}

	var sb strings.Builder
	sb.WriteString("---\nname: ")
	sb.WriteString(name)
	sb.WriteString("\n")
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(body))
	sb.WriteString("\n")
	return writeScheduleRaw(name, sb.String())
}

func writeScheduleRaw(name, content string) error {
	dir := filesystem.ScheduleSkillDir(name)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", dir, err)
	}

	path := filesystem.ScheduleSkillPath(name)
	if err := go_pkg_filesystem.WriteFile(path, content, 0644); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", path, err)
	}
	return nil
}

func GetSchedule(name string) (string, error) {
	result, err := ReadSchedule(name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(bodyRegex.ReplaceAllString(result, "")), nil
}

func TrashSchedule(ctx context.Context, name string, meta historystore.Meta) error {
	dir := filesystem.ScheduleSkillDir(name)
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return nil
	}

	trashPath, err := filesystem.TrashDir(dir, filesystem.ScheduleSkillTrashDir, name)
	if err != nil {
		return err
	}

	meta.Tool = "schedules"
	if err := historystore.RecordDelete(ctx, dir, trashPath, meta); err != nil {
		slog.Debug("historystore.RecordDelete",
			slog.String("path", dir),
			slog.String("error", err.Error()))
	}
	return nil
}
