package searchWeb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	go_utils_http "github.com/pardnchiu/go-utils/http"
)

const (
	path = "https://lite.duckduckgo.com/lite/"
	ttl  = 300
)

var (
	regexLiteAnchor  = regexp.MustCompile(`(?is)<a[^>]*class=['"]result-link['"][^>]*>.*?</a>`)
	regexLiteHref    = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	regexLiteTitle   = regexp.MustCompile(`(?is)<a[^>]*>(.*?)</a>`)
	regexLiteSnippet = regexp.MustCompile(`(?is)<td[^>]*class=['"]result-snippet['"][^>]*>\s*(.*?)\s*</td>`)
	regexTag         = regexp.MustCompile(`<[^>]+>`)
)

type data struct {
	Position    int    `json:"position"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

func handler(ctx context.Context, query, timeRange string) (string, error) {
	hash := sha256.Sum256([]byte(query + "|" + string(timeRange)))
	cacheKey := "search:" + hex.EncodeToString(hash[:])
	db := store.DB(store.DBToolCache)
	if entry, ok := db.Get(cacheKey); ok {
		return entry.Value(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	items, err := fetch(ctx, query, timeRange)
	if err != nil {
		return "", err
	}

	if err = db.Set(cacheKey, items, store.SetDefault, store.TTL(ttl)); err != nil {
		slog.Warn("db.Set",
			slog.String("error", err.Error()))
	}

	return items, nil
}

func fetch(ctx context.Context, query, timeRange string) (string, error) {
	params := map[string]any{
		"q":  query,
		"kl": "tw-tzh",
		"df": timeRange,
	}

	html, status, err := go_utils_http.POST[string](ctx, nil, path, map[string]string{
		"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	}, params, "form")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d", status)
	}

	items := parse(html)
	if len(items) == 0 {
		return "", fmt.Errorf("no result")
	}

	bytes, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(bytes), nil
}

func parse(html string) []data {
	const limit = 10
	anchors := regexLiteAnchor.FindAllStringIndex(html, -1)

	var results []data
	for i, pos := range anchors {
		if len(results) >= limit {
			break
		}
		anchor := html[pos[0]:pos[1]]

		title := ""
		if m := regexLiteTitle.FindStringSubmatch(anchor); len(m) >= 2 {
			title = extractText(m[1])
		}
		if title == "" {
			continue
		}

		href := ""
		if m := regexLiteHref.FindStringSubmatch(anchor); len(m) >= 2 {
			href = extractURL(m[1])
		}
		if href == "" {
			continue
		}

		segEnd := len(html)
		if i+1 < len(anchors) {
			segEnd = anchors[i+1][0]
		}
		desc := ""
		if m := regexLiteSnippet.FindStringSubmatch(html[pos[1]:segEnd]); len(m) >= 2 {
			desc = extractText(m[1])
		}

		results = append(results, data{
			Position:    len(results) + 1,
			Title:       title,
			URL:         href,
			Description: desc,
		})
	}
	return results
}

func extractURL(text string) string {
	if strings.HasPrefix(text, "http") && !strings.Contains(text, "duckduckgo.com") {
		return text
	}

	parsed, err := url.Parse(text)
	if err != nil {
		return ""
	}

	if uddg := parsed.Query().Get("uddg"); uddg != "" {
		if decoded, err := url.QueryUnescape(uddg); err == nil && decoded != "" {
			return decoded
		}
	}
	return ""
}

func extractText(text string) string {
	ddgEntities := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
		"&nbsp;": " ",
	}
	text = regexTag.ReplaceAllString(text, "")

	for entity, char := range ddgEntities {
		text = strings.ReplaceAll(text, entity, char)
	}
	return strings.TrimSpace(text)
}
