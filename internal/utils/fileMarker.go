package utils

import (
	"os"
	"regexp"
	"strings"
)

var (
	fileMarkerRegex = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)
	fileLineRegex   = regexp.MustCompile(`(?m)^FILE:\s+(\S+)\s*$`)
)

func ExtractFileMarkers(text string) (cleanText string, paths []string) {
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

	for _, m := range fileMarkerRegex.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	text = fileMarkerRegex.ReplaceAllString(text, "")

	for _, m := range fileLineRegex.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	text = fileLineRegex.ReplaceAllString(text, "")

	for _, p := range raw {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		paths = append(paths, p)
	}

	cleanText = strings.TrimSpace(text)
	return
}
