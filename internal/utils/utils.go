package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	uuidShortRegex   = regexp.MustCompile(`([0-9a-fA-F]{8})-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	sha256ShortRegex = regexp.MustCompile(`\b([0-9a-fA-F]{8})[0-9a-fA-F]{56}\b`)
)

func ShortenSessionID(sid string) string {
	sid = uuidShortRegex.ReplaceAllString(sid, "$1")
	sid = sha256ShortRegex.ReplaceAllString(sid, "$1")
	return sid
}

func CheckAgentEndpointAlive(ctx context.Context, agent agenttypes.Agent, timeout time.Duration) bool {
	if agent == nil {
		return false
	}

	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, _, err := agent.Send(healthCtx, []go_llm_router.Message{
		{Role: "system", Content: "Reply with only: ok"},
		{Role: "user", Content: "ping"},
	}, nil, go_llm_router.ReasoningNone, fast.Mode(ctx))
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return false
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	return strings.TrimSpace(content) != ""
}

var toolDisplayName = map[string]string{
	"search_web":             "Search Web",
	"find_tools":             "Tools",
	"find_files":             "Find",
	"list_chatbot":           "List Chat",
	"read_files":             "Read",
	"edit_file":              "Edit",
	"fetch_page":             "Fetch",
	"run_command":            "Run",
	"run_command_background": "Background",
	"run_skill":              "Skill",
	"calculate":              "Calc",
	"download_file":          "Download",
	"write_todo":             "Plan",
	"subagents":              "Subagent",
	"chat_history":           "Chat",
	"file_history":           "History",
	"error_history":          "Error",
	"send_to_chatbot":        "Send",
	"http_request":           "Request",
	"schedules":              "Schedule",
	"reasoning_guide":        "Guide",
	"write_result":           "Write",
}

func IsPlugTool(name string) bool {
	return strings.HasPrefix(name, "script_") ||
		strings.HasPrefix(name, "api_") ||
		strings.HasPrefix(name, "ext_")
}

func PlugToolBaseName(name string) string {
	for _, prefix := range []string{"script_", "api_", "ext_"} {
		if base, ok := strings.CutPrefix(name, prefix); ok {
			return base
		}
	}
	return name
}

func IsSubagentInvoke(name, raw string) bool {
	if name != "subagents" {
		return false
	}
	var dic struct {
		Mode string `json:"mode"`
		Task string `json:"task"`
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &dic)
	}
	if mode := strings.TrimSpace(dic.Mode); mode != "" {
		return mode == "invoke"
	}
	return strings.TrimSpace(dic.Task) != ""
}

func ToolName(name string) string {
	if d, ok := toolDisplayName[name]; ok {
		return d
	}
	if IsPlugTool(name) {
		var tag string
		switch {
		case strings.HasPrefix(name, "script_"):
			tag = "Script"
		case strings.HasPrefix(name, "api_"):
			tag = "API"
		case strings.HasPrefix(name, "ext_"):
			tag = "Extension"
		}
		return fmt.Sprintf("Plug Tool(%s %s)", tag, PlugToolBaseName(name))
	}
	return name
}

func FormatToolArgs(name, raw, cwd string) string {
	return formatToolArgs(name, raw, cwd, false)
}

func FormatToolConfirmArgs(name, raw string) string {
	return formatToolArgs(name, raw, "", true)
}

func formatToolArgs(name, raw, cwd string, fullPath bool) string {
	if raw == "" {
		return ""
	}
	var dic map[string]any
	if err := json.Unmarshal([]byte(raw), &dic); err != nil {
		return raw
	}
	if len(dic) == 0 {
		return ""
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := dic[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		return ""
	}
	oneLine := func(s string) string {
		r := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
		return r.Replace(s)
	}
	fileName := func(path string) string {
		path = strings.TrimSpace(path)
		if fullPath {
			return path
		}
		return filepath.Base(path)
	}
	isCwd := func(dir string) bool {
		d := strings.TrimRight(strings.TrimSpace(dir), "/")
		if d == "." || d == "./" || d == "" {
			return true
		}
		c := strings.TrimRight(strings.TrimSpace(cwd), "/")
		return c != "" && d == c
	}
	if IsPlugTool(name) {
		return PlugToolBaseName(name) + " " + raw
	}
	switch name {
	case "subagents":
		label := pick("name", "session_id")
		if label == "" {
			label = "subagent"
		}
		if model := pick("model"); model != "" {
			label = fmt.Sprintf("%s (%s)", label, model)
		}
		if task := pick("task"); task != "" {
			return fmt.Sprintf("%s: %s", label, oneLine(task))
		}
		return label

	case "run_skill":
		if s := pick("skill", "name"); s != "" {
			return s
		}

	case "find_files":
		queries, ok := dic["queries"].([]any)
		if !ok || len(queries) == 0 {
			break
		}
		labels := make([]string, 0, len(queries))
		for _, q := range queries {
			qm, ok := q.(map[string]any)
			if !ok {
				continue
			}
			dir, _ := qm["dir"].(string)
			dir = strings.TrimSpace(dir)
			if dir == "" {
				dir = "."
			}
			if isCwd(dir) {
				dir = "./"
			}
			loc := dir
			if fp, _ := qm["file_pattern"].(string); strings.TrimSpace(fp) != "" {
				loc = strings.TrimRight(dir, "/") + "/" + fp
			}
			if pat, _ := qm["pattern"].(string); strings.TrimSpace(pat) != "" {
				loc += " [" + pat + "]"
			} else if r, ok := qm["recursive"].(bool); ok && r {
				loc += " (recursive)"
			}
			labels = append(labels, loc)
		}
		if len(labels) > 0 {
			return strings.Join(labels, ", ")
		}

	case "read_files":
		files, ok := dic["files"].([]any)
		if !ok || len(files) == 0 {
			break
		}
		paths := make([]string, 0, len(files))
		for _, f := range files {
			fm, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if p, ok := fm["path"].(string); ok && strings.TrimSpace(p) != "" {
				paths = append(paths, fileName(p))
			}
		}
		if len(paths) > 0 {
			return strings.Join(paths, ", ")
		}

	case "edit_file", "open_file":
		if s := pick("path"); s != "" {
			return fileName(s)
		}
		if s := pick("pattern"); s != "" {
			return s
		}

	case "file_history":
		var names []string
		if s := pick("path"); s != "" {
			names = append(names, fileName(s))
		}
		if list, ok := dic["paths"].([]any); ok {
			for _, one := range list {
				if s, ok := one.(string); ok && strings.TrimSpace(s) != "" {
					names = append(names, fileName(s))
				}
			}
		}
		if len(names) > 0 {
			return strings.Join(names, ", ")
		}

	case "search_web":
		if q := pick("query", "keyword"); q != "" {
			if tr := pick("time_range", "time"); tr != "" {
				return fmt.Sprintf("%s [%s]", q, tr)
			}
			return q
		}

	case "fetch_yahoo_finance":
		if sym := pick("symbol"); sym != "" {
			if tr := pick("time_range"); tr != "" {
				return fmt.Sprintf("%s (%s)", sym, tr)
			}
			return sym
		}

	case "fetch_page":
		if s := pick("link", "url"); s != "" {
			return s
		}

	case "calculate":
		if s := pick("expression"); s != "" {
			return s
		}

	case "error_history":
		if s := pick("keyword", "hash", "symptom", "action"); s != "" {
			return s
		}

	case "chat_history":
		if s := pick("keyword", "query"); s != "" {
			return s
		}

	case "schedules":
		skill := pick("skill_name")
		t := pick("time")
		if skill != "" && t != "" {
			return fmt.Sprintf("%s %s", t, skill)
		}
		if skill != "" {
			return skill
		}

	case "find_tools":
		if names := joinStrings(dic["names"]); names != "" {
			return names
		}
		if q := pick("query"); q != "" {
			return q
		}
		return ""

	case "reasoning_guide":
		return joinStrings(dic["topics"])

	case "write_todo":
		return todoProgress(dic["todos"])

	case "write_result":
		format := pick("format")
		if format == "" {
			format = "md"
		}
		return filepath.Join(filesystem.OutputDir(), "report-*."+format)

	case "run_command", "run_command_background":
		var p struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(raw), &p); err != nil || len(p.Argv) == 0 {
			return raw
		}
		parts := make([]string, len(p.Argv))
		for i, a := range p.Argv {
			if a == "" || strings.ContainsAny(a, " \t\n\"'\\") {
				parts[i] = strconv.Quote(a)
			} else {
				parts[i] = a
			}
		}
		return strings.Join(parts, " ")
	}
	return raw
}

type PatchHunk struct {
	OldLines []string
	NewLines []string
	Row      int
}

func FormatPatchDiff(raw string) []PatchHunk {
	var multi struct {
		Targets []struct {
			Old    string `json:"old_string"`
			New    string `json:"new_string"`
			Insert string `json:"insert_string"`
			Row    int    `json:"row"`
		} `json:"targets"`
	}
	if json.Unmarshal([]byte(raw), &multi) == nil && len(multi.Targets) > 0 {
		hunks := make([]PatchHunk, 0, len(multi.Targets))
		for _, t := range multi.Targets {
			if t.Insert != "" {
				hunks = append(hunks, PatchHunk{NewLines: splitLines(t.Insert), Row: t.Row})
				continue
			}
			if t.Old == t.New {
				continue
			}
			hunks = append(hunks, PatchHunk{OldLines: splitLines(t.Old), NewLines: splitLines(t.New), Row: t.Row})
		}
		return hunks
	}

	var p struct {
		Old string `json:"old_string"`
		New string `json:"new_string"`
	}
	if json.Unmarshal([]byte(raw), &p) != nil || p.Old == p.New {
		return nil
	}
	return []PatchHunk{{OldLines: splitLines(p.Old), NewLines: splitLines(p.New)}}
}

func FormatWriteDiff(raw string) []string {
	var p struct {
		Content string `json:"content"`
	}
	if json.Unmarshal([]byte(raw), &p) != nil {
		return nil
	}
	return splitLines(p.Content)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var fileMarkerRegex = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)

func StripFileMarkers(str string) string {
	if !strings.Contains(str, "[SEND_FILE:") {
		return str
	}
	return strings.TrimSpace(fileMarkerRegex.ReplaceAllString(str, ""))
}

func ExtractFileMarkers(str string) (cleanText string, paths []string) {
	seen := map[string]bool{}
	var raw []string
	collect := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		raw = append(raw, path)
	}

	for _, m := range fileMarkerRegex.FindAllStringSubmatch(str, -1) {
		collect(m[1])
	}
	str = fileMarkerRegex.ReplaceAllString(str, "")

	for _, p := range raw {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		paths = append(paths, p)
	}

	cleanText = strings.TrimSpace(str)
	return
}
