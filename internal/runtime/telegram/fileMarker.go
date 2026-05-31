package telegram

import (
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/utils"
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}

func extractFileMarkers(str string) (cleanText string, photoPaths []string, docPaths []string) {
	cleanText, paths := utils.ExtractFileMarkers(str)
	for _, p := range paths {
		if imageExts[strings.ToLower(filepath.Ext(p))] {
			photoPaths = append(photoPaths, p)
			continue
		}
		docPaths = append(docPaths, p)
	}
	return
}
