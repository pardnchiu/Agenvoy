package discord

import (
	"regexp"
	"strings"
)

var (
	fileMarkerRegex = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)
	fileLineRegex   = regexp.MustCompile(`(?m)^FILE:\s+(\S+)\s*$`)
)

func extractFileMarkers(text string) (cleanText string, paths []string) {
	seen := map[string]bool{}
	collect := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	for _, m := range fileMarkerRegex.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	text = fileMarkerRegex.ReplaceAllString(text, "")

	for _, m := range fileLineRegex.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	text = fileLineRegex.ReplaceAllString(text, "")

	cleanText = strings.TrimSpace(text)
	return
}
