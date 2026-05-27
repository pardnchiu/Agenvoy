package filesystem

import (
	"encoding/json"
	"fmt"
	"log/slog"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/configs"
)

// * Runtime limits, loaded once from ~/.config/agenvoy/config.json `limits` section.
// * Defaults below are the only fallback; env vars are no longer read.
var (
	Port                       = "17989"
	MaxToolIterations          = 16
	MaxSkillIterations         = 128
	MaxEmptyResponses          = 8
	MaxRetry                   = 3
	AgentSendTimeoutSec        = 600
	MaxHistoryMessages         = 16
	MaxSessionTasks            = 3
	MaxSubagentTimeoutMin      = 10
	MaxExternalAgentTimeoutMin = 10
)

type DeniedConfig struct {
	Dirs       []string `json:"dirs"`
	Files      []string `json:"files"`
	Prefixes   []string `json:"prefixes"`
	Extensions []string `json:"extensions"`
}

var (
	DeniedMap      DeniedConfig
	DeniedMapBytes []byte
	WhiteList      []string
)

const (
	hardCapMaxSessionTasks            = 10
	hardCapMaxSubagentTimeoutMin      = 60
	hardCapMaxExternalAgentTimeoutMin = 60
)

type RuntimeLimits struct {
	Port                       string `json:"port,omitempty"`
	MaxToolIterations          int    `json:"max_tool_iterations,omitempty"`
	MaxSkillIterations         int    `json:"max_skill_iterations,omitempty"`
	MaxEmptyResponses          int    `json:"max_empty_responses,omitempty"`
	MaxRetry                   int    `json:"max_same_payload_retry,omitempty"`
	AgentSendTimeoutSec        int    `json:"agent_send_timeout_seconds,omitempty"`
	MaxHistoryMessages         int    `json:"max_history_messages,omitempty"`
	MaxSessionTasks            int    `json:"max_session_tasks,omitempty"`
	MaxSubagentTimeoutMin      int    `json:"max_subagent_timeout_min,omitempty"`
	MaxExternalAgentTimeoutMin int    `json:"max_external_agent_timeout_min,omitempty"`
}

// LoadRuntime reads the `limits` section from config.json, fills missing fields
// with defaults, writes back if anything was missing, and assigns the resolved
// values to package vars. Must be called after Init().
func LoadRuntime() error {
	if ConfigPath == "" {
		return fmt.Errorf("filesystem.LoadRuntime: ConfigPath not initialized (call Init first)")
	}

	raw := map[string]json.RawMessage{}
	if go_pkg_filesystem_reader.Exists(ConfigPath) {
		loaded, err := go_pkg_filesystem.ReadJSON[map[string]json.RawMessage](ConfigPath)
		if err != nil {
			return fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
		}
		raw = loaded
	}

	var limits RuntimeLimits
	if data, ok := raw["limits"]; ok && len(data) > 0 {
		if err := json.Unmarshal(data, &limits); err != nil {
			return fmt.Errorf("json.Unmarshal limits: %w", err)
		}
	}

	changed := false
	if limits.Port == "" {
		limits.Port = Port
		changed = true
	}
	Port = limits.Port

	if limits.MaxToolIterations <= 0 {
		limits.MaxToolIterations = MaxToolIterations
		changed = true
	}
	MaxToolIterations = limits.MaxToolIterations

	if limits.MaxSkillIterations <= 0 {
		limits.MaxSkillIterations = MaxSkillIterations
		changed = true
	}
	MaxSkillIterations = limits.MaxSkillIterations

	if limits.MaxEmptyResponses <= 0 {
		limits.MaxEmptyResponses = MaxEmptyResponses
		changed = true
	}
	MaxEmptyResponses = limits.MaxEmptyResponses

	if limits.MaxRetry <= 0 {
		limits.MaxRetry = MaxRetry
		changed = true
	}
	MaxRetry = limits.MaxRetry

	if limits.AgentSendTimeoutSec <= 0 {
		limits.AgentSendTimeoutSec = AgentSendTimeoutSec
		changed = true
	}
	AgentSendTimeoutSec = limits.AgentSendTimeoutSec

	if limits.MaxHistoryMessages <= 0 {
		limits.MaxHistoryMessages = MaxHistoryMessages
		changed = true
	}
	MaxHistoryMessages = limits.MaxHistoryMessages

	if limits.MaxSessionTasks <= 0 {
		limits.MaxSessionTasks = MaxSessionTasks
		changed = true
	}
	MaxSessionTasks = min(hardCapMaxSessionTasks, limits.MaxSessionTasks)

	if limits.MaxSubagentTimeoutMin <= 0 {
		limits.MaxSubagentTimeoutMin = MaxSubagentTimeoutMin
		changed = true
	}
	MaxSubagentTimeoutMin = min(hardCapMaxSubagentTimeoutMin, limits.MaxSubagentTimeoutMin)

	if limits.MaxExternalAgentTimeoutMin <= 0 {
		limits.MaxExternalAgentTimeoutMin = MaxExternalAgentTimeoutMin
		changed = true
	}
	MaxExternalAgentTimeoutMin = min(hardCapMaxExternalAgentTimeoutMin, limits.MaxExternalAgentTimeoutMin)

	if err := json.Unmarshal(configs.DeniedMap, &DeniedMap); err != nil {
		return fmt.Errorf("embedded denied_map: %w", err)
	}
	if err := json.Unmarshal(configs.WhiteList, &WhiteList); err != nil {
		return fmt.Errorf("embedded white_list: %w", err)
	}
	if data, ok := raw["denied_map"]; ok && len(data) > 0 {
		var user DeniedConfig
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("json.Unmarshal denied_map: %w", err)
		}
		DeniedMap.Dirs = merge(DeniedMap.Dirs, user.Dirs)
		DeniedMap.Files = merge(DeniedMap.Files, user.Files)
		DeniedMap.Prefixes = merge(DeniedMap.Prefixes, user.Prefixes)
		DeniedMap.Extensions = merge(DeniedMap.Extensions, user.Extensions)
	}
	if data, ok := raw["white_list"]; ok && len(data) > 0 {
		var user []string
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("json.Unmarshal white_list: %w", err)
		}
		WhiteList = merge(WhiteList, user)
	}

	deniedBytes, err := json.Marshal(DeniedMap)
	if err != nil {
		return fmt.Errorf("json.Marshal denied_map: %w", err)
	}
	DeniedMapBytes = deniedBytes

	go_pkg_sandbox.New(DeniedMapBytes)
	if err := go_pkg_filesystem.New(go_pkg_filesystem.Policy{
		DeniedMap:   DeniedMapBytes,
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
		return fmt.Errorf("json.Marshal limits: %w", err)
	}
	raw["limits"] = limitsRaw
	if err := go_pkg_filesystem.CheckDir(AgenvoyDir, true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}
	if err := go_pkg_filesystem.WriteJSON(ConfigPath, raw, false); err != nil {
		return fmt.Errorf("go_pkg_filesystem.WriteJSON: %w", err)
	}
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
