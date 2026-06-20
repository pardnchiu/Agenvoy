package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
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

func CheckAgentEndpointAlive(ctx context.Context, agent agentTypes.Agent, timeout time.Duration) bool {
	if agent == nil {
		return false
	}

	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := agent.Send(healthCtx, []agentTypes.Message{
		{Role: "system", Content: "Reply with only: ok"},
		{Role: "user", Content: "ping"},
	}, nil)
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return false
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	return strings.TrimSpace(content) != ""
}

var toolDisplayName = map[string]string{
	"search_chat_history":   "Search Chat",
	"search_error_history":  "Search Error",
	"search_google_news":    "Search News",
	"search_web":            "Search Web",
	"search_rag":            "Search RAG",
	"search_files":          "Search Files",
	"search_tools":          "Search Tools",
	"list_recent_tool_call": "Recent Calls",
	"read_tool_call":        "Read Cache",
	"list_rag":              "List RAG",
	"list_files":            "List Files",
	"list_tools":            "List Tools",
	"list_chatbot":          "List Chat",
	"list_schedule":         "List Schedule",
	"read_file":             "Read",
	"write_file":            "Write",
	"patch_file":            "Patch",
	"glob_files":            "Glob",
	"fetch_page":            "Fetch",
	"run_command":           "Run",
	"run_skill":             "Skill",
	"calculate":             "Calc",
	"download_file":         "Download",
	"generate_plan":         "Plan",
	"generate_image":        "Image",
	"invoke_subagent":       "Subagent",
	"git_log":               "Git Log",
	"git_rollback":          "Rollback",
	"read_error":            "Read",
	"read_log":              "Read",
	"remember_error":        "Remember",
	"report_error":          "Report",
	"format_chatbot":        "Format",
	"send_to_chatbot":       "Send",
	"send_http_request":     "Request",
	"transcribe_media":      "Transcribe",
	"add_schedule":          "Add Schedule",
	"patch_schedule":        "Patch Schedule",
	"remove_schedule":       "Remove Schedule",
}

func ToolName(name string) string {
	if d, ok := toolDisplayName[name]; ok {
		return d
	}
	return name
}

func FormatToolArgs(name, raw, cwd string) string {
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
	isCwd := func(dir string) bool {
		d := strings.TrimRight(strings.TrimSpace(dir), "/")
		if d == "." || d == "./" || d == "" {
			return true
		}
		c := strings.TrimRight(strings.TrimSpace(cwd), "/")
		return c != "" && d == c
	}
	switch name {
	case "invoke_subagent":
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

	case "list_files":
		dir := pick("dir", "path")
		if dir == "" {
			break
		}
		if r, ok := dic["recursive"].(bool); ok && r {
			return dir + " (recursive)"
		}
		return dir

	case "read_file", "write_file", "patch_file", "glob_files":
		if s := pick("path", "pattern"); s != "" {
			return s
		}

	case "search_files":
		dir := strings.TrimSpace(pick("dir"))
		if dir == "" {
			dir = "."
		}
		if isCwd(dir) {
			dir = "./"
		}
		loc := dir
		if fp := strings.TrimSpace(pick("file_pattern")); fp != "" {
			loc = strings.TrimRight(dir, "/") + "/" + fp
		}
		if pat := pick("pattern"); pat != "" {
			return loc + " [" + pat + "]"
		}
		return loc

	case "search_web", "search_google_news":
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

	case "remember_error":
		if s := pick("symptom", "cause", "action"); s != "" {
			return s
		}

	case "read_tool_call":
		if s := pick("id"); s != "" {
			return s
		}

	case "search_rag":
		db := pick("db")
		mode := pick("mode")
		q := pick("q", "query")
		if q == "" {
			break
		}
		var parts []string
		if db != "" {
			parts = append(parts, db)
		}
		if mode != "" {
			parts = append(parts, mode)
		}
		parts = append(parts, fmt.Sprintf("%q", q))
		if limit, ok := dic["limit"]; ok {
			if n, ok := limit.(float64); ok && n > 0 {
				parts = append(parts, fmt.Sprintf("[%d]", int(n)))
			}
		}
		return strings.Join(parts, " ")

	case "search_error_history", "search_chat_history":
		if s := pick("keyword", "query"); s != "" {
			return s
		}

	case "add_schedule", "patch_schedule":
		skill := pick("skill_name")
		t := pick("time")
		if skill != "" && t != "" {
			return fmt.Sprintf("%s %s", t, skill)
		}
		if skill != "" {
			return skill
		}

	case "remove_schedule":
		if skill := pick("skill_name"); skill != "" {
			return skill
		}

	case "run_command":
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

const maxDiffLines = 8

func FormatPatchDiff(raw string) (oldLines, newLines []string) {
	var p struct {
		Old string `json:"old_string"`
		New string `json:"new_string"`
	}
	if json.Unmarshal([]byte(raw), &p) != nil {
		return nil, nil
	}
	oldLines = splitTruncate(p.Old)
	newLines = splitTruncate(p.New)
	return
}

func FormatWriteDiff(raw string) []string {
	var p struct {
		Content string `json:"content"`
	}
	if json.Unmarshal([]byte(raw), &p) != nil {
		return nil
	}
	return splitTruncate(p.Content)
}

func splitTruncate(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxDiffLines {
		lines = append(lines[:maxDiffLines], fmt.Sprintf("… +%d lines", len(lines)-maxDiffLines))
	}
	return lines
}

var fileMarkerRegex = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)

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
