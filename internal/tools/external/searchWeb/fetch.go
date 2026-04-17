package searchWeb

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	go_utils_http "github.com/pardnchiu/go-utils/http"
)

const path = "https://lite.duckduckgo.com/lite/"

var (
	regexLiteAnchor  = regexp.MustCompile(`(?is)<a[^>]*class=['"]result-link['"][^>]*>.*?</a>`)
	regexLiteHref    = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	regexLiteTitle   = regexp.MustCompile(`(?is)<a[^>]*>(.*?)</a>`)
	regexLiteSnippet = regexp.MustCompile(`(?is)<td[^>]*class=['"]result-snippet['"][^>]*>\s*(.*?)\s*</td>`)
	regexTag         = regexp.MustCompile(`<[^>]+>`)
)

func fetch(ctx context.Context, query string, timeRange TimeRange) ([]ResultData, error) {
	params := map[string]any{
		"q":  query,
		"kl": "tw-tzh",
	}

	switch timeRange {
	case TimeRange1d:
		params["df"] = "d"
	case TimeRange7d:
		params["df"] = "w"
	case TimeRangeMonth:
		params["df"] = "m"
	case TimeRangeYear:
		params["df"] = "y"
	}

	html, status, err := go_utils_http.POST[string](ctx, nil, path, map[string]string{
		"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Accept-Language": "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	}, params, "form")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d", status)
	}

	results := parse(html)
	if len(results) == 0 {
		return nil, fmt.Errorf("parse: %s", query)
	}
	return results, nil
}

func parse(html string) []ResultData {
	const limit = 10
	anchors := regexLiteAnchor.FindAllStringIndex(html, -1)

	var results []ResultData
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

		results = append(results, ResultData{
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
