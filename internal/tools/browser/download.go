package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func Download(href, saveTo string) (string, error) {
	if href == "" {
		return "", fmt.Errorf("href is required")
	}

	if saveTo == "" {
		return "", fmt.Errorf("saveTo is required")
	}

	// if dir, err := utils.GetConfigDir("tools", "browser", "5xx"); err == nil {
	// 	clean(dir.Home, skippedExpired)
	// }
	cached5xxDir := filepath.Join(filesystem.ToolsDir, "browser", "5xx")
	clean(cached5xxDir, skippedExpired)

	if isSkipped(href) {
		return skippedMessage(href), nil
	}

	hash := sha256.Sum256([]byte(href + "|download"))
	cacheKey := hex.EncodeToString(hash[:])
	// configDir, err := utils.GetConfigDir("tools", "browser", "cached")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }
	cached := filepath.Join(filesystem.ToolsDir, "browser", "cached")

	clean(cached, cacheExpired)
	cachePath := filepath.Join(cached, cacheKey+".md")
	var content string
	if _, err := os.Stat(cachePath); err == nil {
		if cached, err := os.ReadFile(cachePath); err == nil {
			content = string(cached)
		}
	}

	if content == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		browser, err := newBrowser()
		if err != nil {
			return "", err
		}
		defer browser.MustClose()

		page, err := fetch(ctx, browser, href)
		if err != nil {
			status := 503
			var fetchErr *FetchError
			if errors.As(err, &fetchErr) {
				status = fetchErr.Status
			}
			addToSkippedMap(href, status)
			return skippedMessage(href), nil
		}
		defer page.MustClose()

		html, err := page.HTML()
		if err != nil {
			return "", fmt.Errorf("page.HTML: %w", err)
		}

		data, err := extract(href, html)
		if err != nil || strings.TrimSpace(data.Markdown) == "" {
			addToSkippedMap(href, 0)
			return skippedMessage(href), nil
		}

		content = data.Markdown
		if err := filesystem.WriteFile(cachePath, content, 0644); err != nil {
			slog.Warn("utils.WriteFile",
				slog.String("error", err.Error()))
		}
	}

	if err := os.MkdirAll(filepath.Dir(saveTo), 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	if err := filesystem.WriteFile(saveTo, content, 0644); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", err)
	}

	return fmt.Sprintf("Downloaded %d chars to %s", len(content), saveTo), nil
}
