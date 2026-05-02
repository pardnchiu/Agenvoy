package fetchPage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_rod "github.com/pardnchiu/go-pkg/rod"

	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	cacheExpired      = 1 * time.Hour
	skippedExpired    = 12 * time.Hour
	fetchTimeout      = 5 * time.Second
	maxMarkdownLength = 100 << 10
)

func registFetchPage() {
	toolRegister.Regist(toolRegister.Def{

		Name:       "fetch_page",
		ReadOnly:   true,
		Concurrent: true,
		Description: "Fetch a web page and return its content as Markdown without saving it locally.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "The full URL of the page to fetch (must include https://)",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "Keep hyperlinks from the same domain (useful for document research tasks that require recursively following subpages).",
					"default":     false,
				},
			},
			"required": []string{
				"link",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Link      string `json:"link"`
				KeepLinks bool   `json:"keep_links"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", fmt.Errorf("link is required")
			}
			return handler(link, params.KeepLinks, nil)
		},
	})
}

func handler(link string, keepLinks bool, saveTo *string) (string, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("url.Parse: %w", err)
	}
	if err := validateURL(link); err != nil {
		return "", err
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		link = parsed.String()
	}

	if hit, status, title := isSkipped(link); hit {
		return skippedMessage(link, status, title), nil
	}

	cacheVariant := "|text"
	if keepLinks {
		cacheVariant = "|links"
	}
	hash := sha256.Sum256([]byte(link + cacheVariant))
	cacheKey := "page:" + hex.EncodeToString(hash[:])
	db := torii.DB(torii.DBToolCache)
	var full string
	if entry, ok := db.Get(cacheKey); ok {
		if saveTo == nil {
			return truncateResult(entry.Value()), nil
		}
		full = entry.Value()
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		result, err := go_pkg_rod.Fetch(ctx, link, fetchTimeout, &go_pkg_rod.FetchOption{
			MaxLength: maxMarkdownLength,
			KeepLinks: keepLinks,
		})
		if err != nil {
			status := 503
			title := ""
			var fe *go_pkg_rod.FetchError
			if errors.As(err, &fe) {
				status = fe.Status
			}
			if result != nil {
				title = result.Title
			}
			addToSkippedMap(link, status, title)
			return skippedMessage(link, status, title), nil
		}

		if isPage4xx(result.Title, result.FinalURL) {
			addToSkippedMap(link, 404, result.Title)
			return skippedMessage(link, 404, result.Title), nil
		}

		if strings.TrimSpace(result.Markdown) == "" {
			addToSkippedMap(link, 0, result.Title)
			return skippedMessage(link, 0, result.Title), nil
		}

		full = buildFrontmatter(result)
		if err = db.Set(cacheKey, full, torii.SetDefault, torii.TTL(int64(cacheExpired.Seconds()))); err != nil {
			slog.Warn("db.Set",
				slog.String("error", err.Error()))
		}
	}

	if saveTo != nil {
		if err := go_pkg_filesystem.CheckDir(filepath.Dir(*saveTo), true); err != nil {
			return "", fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
		}
		if err := go_pkg_filesystem.WriteFile(*saveTo, full, 0644); err != nil {
			return "", fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
		}
		return fmt.Sprintf("Downloaded %d chars to %s", len(full), *saveTo), nil
	}
	return truncateResult(full), nil
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

func truncateResult(result string) string {
	if len(result) >= maxMarkdownLength {
		result = result[:maxMarkdownLength] + "\n\n[Content truncated due to length...]"
	}
	return fmt.Sprintf("Web page content:\n---\n%s\n---", result)
}

func buildFrontmatter(r *go_pkg_rod.FetchResult) string {
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
