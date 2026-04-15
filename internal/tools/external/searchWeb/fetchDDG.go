package searchWeb

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	go_utils_http "github.com/pardnchiu/go-utils/http"
)

var (
	regexLink    = regexp.MustCompile(`(?s)<div[^>]+class="[^"]*result[^"]*results_links[^"]*"[^>]*>(.*?)</div>\s*</div>\s*</div>`)
	regexA       = regexp.MustCompile(`(?i)<a[^>]+class="[^"]*result__a[^"]*"[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	regexSnippet = regexp.MustCompile(`(?i)<a[^>]+class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</a>`)
	regexTag     = regexp.MustCompile(`<[^>]+>`)
)

func fetchDDG(ctx context.Context, query string, timeRange TimeRange) ([]ResultData, error) {
	params := map[string]any{
		"q":  query,
		"kl": "tw-tzh",
		"kp": "-2",
		"k1": "-1",
	}

	switch timeRange {
	case TimeRange1h, TimeRange3h, TimeRange6h, TimeRange12h, TimeRange1d:
		params["df"] = "d"
	case TimeRange7d:
		params["df"] = "w"
	case TimeRangeMonth:
		params["df"] = "m"
	case TimeRangeYear:
		params["df"] = "y"
	}

	html, _, err := go_utils_http.POST[string](ctx, nil, "https://html.duckduckgo.com/html/", map[string]string{
		"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	}, params, "form")
	if err != nil {
		return nil, fmt.Errorf("http.POST: %w", err)
	}

	results := parse(html)
	if len(results) == 0 {
		return nil, fmt.Errorf("parse: %s", query)
	}
	return results, nil
}

func parse(html string) []ResultData {
	const limit = 10
	matches := regexLink.FindAllString(html, -1)

	var results []ResultData
	for _, match := range matches {
		if len(results) >= limit {
			break
		}

		result := extract(match, len(results)+1)
		if result == nil {
			continue
		}
		results = append(results, *result)
	}
	return results
}

func extract(block string, position int) *ResultData {
	matches := regexA.FindStringSubmatch(block)
	if len(matches) < 3 {
		return nil
	}

	title := extractText(matches[2])
	if title == "" {
		return nil
	}

	url := extractURL(matches[1])
	if url == "" {
		return nil
	}

	desc := ""
	if matches = regexSnippet.FindStringSubmatch(block); len(matches) >= 2 {
		desc = extractText(matches[1])
	}

	return &ResultData{
		Position:    position,
		Title:       title,
		URL:         url,
		Description: desc,
	}
}

func extractURL(text string) string {
	if strings.HasPrefix(text, "http") && !strings.Contains(text, "duckduckgo.com") {
		return text
	}

	parse, err := url.Parse(text)
	if err != nil {
		return ""
	}

	if uddg := parse.Query().Get("uddg"); uddg != "" {
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
