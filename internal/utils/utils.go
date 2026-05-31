package utils

import (
	"context"
	"os"
	"regexp"
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
