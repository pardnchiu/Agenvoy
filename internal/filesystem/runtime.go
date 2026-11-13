package filesystem

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var (
	SensitivePath   go_pkg_filesystem.SensitiveDic
	DeniedCommand   []string
	DeniedPath      []string
	NetWhiteList    []string
	ReadOnlyCommand []string
	ReadablePath    []string
	ConfigReplyLang = ReplyLangAuto
	ConfigOutputDir string
)

const ReplyLangAuto = "auto"

const Port = "17989"

type RuntimeLimits struct {
	MaxToolIterations   int `json:"max_tool_iterations,omitempty"`
	AgentSendTimeoutSec int `json:"agent_send_timeout_seconds,omitempty"`
	MaxHistoryMessages  int `json:"max_history_messages,omitempty"`
	MaxHistoryBytes     int `json:"max_history_bytes,omitempty"`
}

func LoadRuntime() error {
	if ConfigPath == "" {
		return utilsmsg.New(utilsmsg.Internal, "filesystem.LoadRuntime", "ConfigPath not initialized (call Init first)")
	}

	raw := map[string]json.RawMessage{}
	if go_pkg_filesystem_reader.Exists(ConfigPath) {
		loaded, err := go_pkg_filesystem.ReadJSON[map[string]json.RawMessage](ConfigPath)
		if err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "go_pkg_filesystem.ReadJSON", ConfigPath, err)
		}
		raw = loaded
	}

	var limits RuntimeLimits
	if data, ok := raw["limits"]; ok && len(data) > 0 {
		if err := json.Unmarshal(data, &limits); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "limits", err)
		}
	}

	changed := false

	configs.MAX_TOOL_ITERATIONS = max(limits.MaxToolIterations, configs.MAX_TOOL_ITERATIONS)
	configs.AGENT_SEND_TIMEOUT_SEC = max(limits.AgentSendTimeoutSec, configs.AGENT_SEND_TIMEOUT_SEC)
	configs.MAX_HISTORY_MESSAGES = max(limits.MaxHistoryMessages, configs.MAX_HISTORY_MESSAGES)
	configs.MAX_HISTORY_BYTES = max(limits.MaxHistoryBytes, configs.MAX_HISTORY_BYTES)

	ConfigReplyLang = ReplyLangAuto
	if data, ok := raw["reply_lang"]; ok && len(data) > 0 {
		var user string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "reply_lang", err)
		}
		if user = strings.TrimSpace(user); user != "" {
			ConfigReplyLang = user
		}
	}
	langRaw, err := json.Marshal(ConfigReplyLang)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "reply_lang", err)
	}
	if string(raw["reply_lang"]) != string(langRaw) {
		raw["reply_lang"] = langRaw
		changed = true
	}

	ConfigOutputDir = ""
	if data, ok := raw["output_dir"]; ok && len(data) > 0 {
		var user string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "output_dir", err)
		}
		ConfigOutputDir = strings.TrimSpace(user)
	} else {
		raw["output_dir"] = json.RawMessage(`""`)
		changed = true
	}

	if data, ok := raw["model_tag"]; !ok || len(data) == 0 || string(data) == "null" {
		raw["model_tag"] = json.RawMessage(`{}`)
		changed = true
	}

	SensitivePath = go_pkg_filesystem.SensitiveDic{}
	if data, ok := raw["sensitive_path"]; ok && len(data) > 0 {
		if err := json.Unmarshal(data, &SensitivePath); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "sensitive_path", err)
		}
	}
	if data, ok := raw["denied_command"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "denied_command", err)
		}
		DeniedCommand = merge(nil, user)
	}

	if data, ok := raw["denied_path"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "denied_path", err)
		}
		DeniedPath = normalizePaths("denied_path", user)
	}

	ReadablePath = nil
	if data, ok := raw["readable_path"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "readable_path", err)
		}
		ReadablePath = normalizePaths("readable_path", user)
	}

	if data, ok := raw["net_white_list"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "net_white_list", err)
		}
		NetWhiteList = merge(nil, user)
	}

	if err := json.Unmarshal(configs.ReadOnlyCommand, &ReadOnlyCommand); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Unmarshal", "embedded read_only_command", err)
	}
	if data, ok := raw["read_only_command"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "read_only_command", err)
		}
		ReadOnlyCommand = merge(ReadOnlyCommand, user)
	}

	if err := applySandboxDenied(); err != nil {
		return err
	}

	if err := go_pkg_filesystem.New(go_pkg_filesystem.Policy{
		ExcludeList: configs.ExcludeList,
	}); err != nil {
		slog.Warn("go_pkg_filesystem New",
			slog.String("error", err.Error()))
	}

	if !changed {
		return nil
	}

	limitsRaw, err := json.Marshal(limits)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "limits", err)
	}
	raw["limits"] = limitsRaw
	if err := go_pkg_filesystem.CheckDir(AgenvoyDir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", AgenvoyDir, err)
	}
	if err := go_pkg_filesystem.WriteJSON(ConfigPath, raw, false); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteJSON", ConfigPath, err)
	}
	return nil
}

func normalizePaths(field string, list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	root := string(filepath.Separator)
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !filepath.IsAbs(entry) && entry != "~" && !strings.HasPrefix(entry, "~/") {
			slog.Warn("[Invalid] "+field+": entry dropped, needs an absolute path or ~/",
				slog.String("entry", entry))
			continue
		}
		one := go_pkg_utils.AbsPath("", entry)
		if !filepath.IsAbs(one) {
			slog.Warn("[Invalid] "+field+": entry dropped, cannot be resolved",
				slog.String("entry", entry))
			continue
		}
		if one == root {
			slog.Warn("[Invalid] "+field+": entry dropped, the filesystem root is not allowed",
				slog.String("entry", entry))
			continue
		}
		if !seen[one] {
			seen[one] = true
			out = append(out, one)
		}

		resolved, err := go_pkg_filesystem.RealPath(one)
		if err != nil || resolved == root || seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
	}
	return out
}

func applySandboxDenied() error {
	if len(DeniedPath) == 0 {
		return nil
	}
	payload := struct {
		Dirs  []string `json:"dirs"`
		Files []string `json:"files"`
	}{}
	for _, one := range DeniedPath {
		if go_pkg_filesystem_reader.IsFile(one) {
			payload.Files = append(payload.Files, one)
			continue
		}
		payload.Dirs = append(payload.Dirs, one)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "denied_path", err)
	}
	go_pkg_sandbox.New(raw)
	return nil
}

func merge(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, v := range base {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, v := range extra {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
