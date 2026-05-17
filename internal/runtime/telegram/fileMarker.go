package telegram

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	fileMarkerRegex = regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)
	fileLineRegex   = regexp.MustCompile(`(?m)^FILE:\s+(\S+)\s*$`)
	imageExts       = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
)

func extractFileMarkers(text string) (cleanText string, photoPaths []string, docPaths []string) {
	seen := map[string]bool{}
	collect := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		if imageExts[strings.ToLower(filepath.Ext(path))] {
			photoPaths = append(photoPaths, path)
			return
		}
		docPaths = append(docPaths, path)
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
