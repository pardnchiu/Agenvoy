package fetchPage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	goutilsrod "github.com/pardnchiu/go-utils/rod"
)

func Download(href, saveTo string) (string, error) {
	if href == "" {
		return "", fmt.Errorf("href is required")
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("url.Parse: %w", err)
	}
	if err := validateURL(href); err != nil {
		return "", err
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		href = parsed.String()
	}

	if saveTo == "" {
		saveTo = defaultDownloadPath(href)
	}

	if isSkipped(href) {
		return skippedMessage(href), nil
	}

	hash := sha256.Sum256([]byte(href + "|download"))
	cacheKey := "page:" + hex.EncodeToString(hash[:])

	db := store.DB(store.DBToolCache)
	var content string
	if entry, ok := db.Get(cacheKey); ok {
		content = entry.Value()
	}

	if content == "" {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		data, err := fetch(ctx, href, false)
		if err != nil {
			status := 503
			var fe *goutilsrod.FetchError
			if errors.As(err, &fe) {
				status = fe.Status
			}
			addToSkippedMap(href, status)
			return skippedMessage(href), nil
		}

		if strings.TrimSpace(data.Markdown) == "" {
			addToSkippedMap(href, 0)
			return skippedMessage(href), nil
		}

		content = buildFrontmatter(data)
		if err := db.Set(cacheKey, content, store.SetDefault, store.TTL(int64(cacheExpired.Seconds()))); err != nil {
			slog.Warn("db.Set",
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

func defaultDownloadPath(href string) string {
	name := "page"
	if u, err := url.Parse(href); err == nil {
		seg := strings.TrimSuffix(filepath.Base(u.Path), "/")
		if seg != "" && seg != "." {
			name = seg
		} else if u.Host != "" {
			name = u.Host
		}
	}
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return filepath.Join(filesystem.DownloadDir, name)
}
