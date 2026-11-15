package utils

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
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
