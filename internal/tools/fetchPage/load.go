package fetchPage

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	go_utils_rod "github.com/pardnchiu/go-utils/rod"
)

//go:embed embed/skipped.md
var skippedPrompt string

const (
	cacheExpired      = 1 * time.Hour
	skippedExpired    = 12 * time.Hour
	fetchTimeout      = 30 * time.Second
	maxMarkdownLength = 100 << 10
)

func skippedMessage(href string) string {
	tmpl, err := template.New("skipped").Parse(skippedPrompt)
	if err != nil {
		return href
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, struct{ Href string }{href}); err != nil {
		return href
	}
	return sb.String()
}

func validateURL(href string) error {
	if len(href) > 2048 {
		return fmt.Errorf("url too long: max 2048 chars")
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return fmt.Errorf("url.Parse: %w", err)
	}
	if parsed.User != nil {
		return fmt.Errorf("url must not contain credentials")
	}
	if !strings.Contains(parsed.Hostname(), ".") {
		return fmt.Errorf("url must have a valid hostname")
	}
	return nil
}

func Load(href string, keepLinks bool) (string, error) {
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

	if isSkipped(href) {
		return skippedMessage(href), nil
	}

	cacheVariant := "|text"
	if keepLinks {
		cacheVariant = "|links"
	}
	hash := sha256.Sum256([]byte(href + cacheVariant))
	cacheKey := "page:" + hex.EncodeToString(hash[:])

	db := store.DB(store.DBFetchPage)
	if entry, ok := db.Get(cacheKey); ok {
		return formatForAgent(truncate(entry.Value)), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	result, err := fetch(ctx, href, keepLinks)
	if err != nil {
		status := 503
		var fe *go_utils_rod.FetchError
		if errors.As(err, &fe) {
			status = fe.Status
		}
		addToSkippedMap(href, status)
		return skippedMessage(href), nil
	}

	if detect4xx(result.Title) {
		addToSkippedMap(href, 404)
		return skippedMessage(href), nil
	}

	if strings.TrimSpace(result.Markdown) == "" {
		addToSkippedMap(href, 0)
		return skippedMessage(href), nil
	}

	full := buildFrontmatter(result)
	if err = db.Set(cacheKey, full, store.SetDefault, store.TTL(int64(cacheExpired.Seconds()))); err != nil {
		slog.Warn("db.Set",
			slog.String("error", err.Error()))
	}
	return formatForAgent(truncate(full)), nil
}

func fetch(ctx context.Context, href string, keepLinks bool) (*go_utils_rod.FetchResult, error) {
	return go_utils_rod.Fetch(ctx, href, &go_utils_rod.FetchOption{
		Timeout:   fetchTimeout,
		MaxLength: maxMarkdownLength,
		KeepLinks: keepLinks,
	})
}

func detect4xx(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "404", "403", "not found", "page not found",
		"404 not found", "403 forbidden", "access denied",
		"找不到頁面", "頁面不存在", "此頁面不存在":
		return true
	}
	return false
}

func formatForAgent(content string) string {
	return "Web page content:\n---\n" + content + "\n---"
}

func truncate(s string) string {
	if len(s) <= maxMarkdownLength {
		return s
	}
	return s[:maxMarkdownLength] + "\n\n[Content truncated due to length...]"
}

func buildFrontmatter(r *go_utils_rod.FetchResult) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	writeField(&sb, "title", r.Title)
	writeField(&sb, "url", r.Href)
	if r.Author != "" {
		writeField(&sb, "author", r.Author)
	}
	if r.PublishedAt != "" && r.PublishedAt != "0001-01-01T00:00:00Z" {
		writeField(&sb, "published_at", r.PublishedAt)
	}
	if r.Excerpt != "" {
		writeField(&sb, "excerpt", r.Excerpt)
	}
	sb.WriteString("---\n")
	sb.WriteString(r.Markdown)
	return sb.String()
}

func writeField(sb *strings.Builder, key, val string) {
	val = strings.ReplaceAll(val, `"`, `\"`)
	fmt.Fprintf(sb, "%s: \"%s\"\n", key, val)
}
