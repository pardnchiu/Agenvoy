package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	go_browser "github.com/pardnchiu/go-browser"

	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	ddgPath   = "https://html.duckduckgo.com/html/"
	ddgMinGap = 5 * time.Second // prevent status code 202
)

var (
	ddgMu       sync.Mutex
	ddgLastCall time.Time
	cdpForced   atomic.Bool
)

var ddgClient *http.Client

func init() {
	jar, _ := cookiejar.New(nil)
	ddgClient = &http.Client{
		Jar:     jar,
		Timeout: 15 * time.Second,
	}
}

var userAgents = [...]string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
}

var (
	regexAnchor  = regexp.MustCompile(`(?is)<a[^>]*class=['"]result__a['"][^>]*>.*?</a>`)
	regexHref    = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	regexTitle   = regexp.MustCompile(`(?is)<a[^>]*>(.*?)</a>`)
	regexSnippet = regexp.MustCompile(`(?is)<a[^>]*class=['"]result__snippet['"][^>]*>\s*(.*?)\s*</a>`)
	regexTag     = regexp.MustCompile(`<[^>]+>`)
)

type data struct {
	Position    int    `json:"position"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

func handler(ctx context.Context, query, timeRange string, cdp bool) (string, error) {
	if !cdp {
		if err := reserveSlot(ctx); err != nil {
			return "", err
		}
	}

	return fetch(ctx, query, timeRange, cdp)
}

func reserveSlot(ctx context.Context) error {
	ddgMu.Lock()
	defer ddgMu.Unlock()

	if wait := ddgMinGap - time.Since(ddgLastCall); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ddgLastCall = time.Now()
	return nil
}

func browserHeaders() map[string]string {
	ua := userAgents[rand.IntN(len(userAgents))]
	return map[string]string{
		"User-Agent":                ua,
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		"Accept-Encoding":           "identity",
		"Referer":                   "https://html.duckduckgo.com/",
		"Origin":                    "https://html.duckduckgo.com",
		"Sec-Fetch-Site":            "same-origin",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Dest":            "document",
		"Upgrade-Insecure-Requests": "1",
	}
}

func primeCookies(ctx context.Context, headers map[string]string) {
	u, err := url.Parse(ddgPath)
	if err != nil {
		return
	}
	if len(ddgClient.Jar.Cookies(u)) > 0 {
		return
	}
	primeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(primeCtx, "GET", ddgPath, nil)
	if err != nil {
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ddgClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func fetch(ctx context.Context, query, timeRange string, cdp bool) (string, error) {
	if cdp || cdpForced.Load() {
		return fetchCDP(ctx, query, timeRange)
	}

	headers := browserHeaders()
	primeCookies(ctx, headers)

	params := map[string]any{
		"q":  query,
		"kl": "tw-tzh",
		"df": timeRange,
	}

	httpCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	html, status, err := go_pkg_http.POST[string](httpCtx, ddgClient, ddgPath, headers, params, "form")
	if err != nil {
		return "", err
	}
	if status == http.StatusAccepted {
		cdpForced.Store(true)
		slog.Warn("search_web HTTP 202, fallback to CDP (locked)")
		return fetchCDP(ctx, query, timeRange)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d", status)
	}

	items := parse(html)
	if len(items) == 0 {
		return "[]", nil
	}

	items = filterLinks(ctx, items)
	if len(items) == 0 {
		return "[]", nil
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(raw), nil
}

func fetchCDP(ctx context.Context, query, timeRange string) (string, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("kl", "tw-tzh")
	if timeRange != "" {
		params.Set("df", timeRange)
	}
	link := ddgPath + "?" + params.Encode()

	result, err := go_browser.Fetch(ctx, link, 30*time.Second, &go_browser.Option{
		MaxLength: 200 << 10,
		Type:      go_browser.TypeHTML,
	})
	if err != nil {
		return "", fmt.Errorf("go_browser.Fetch: %w", err)
	}

	items := parse(result.Content)
	if len(items) == 0 {
		return "[]", nil
	}

	items = filterLinks(ctx, items)
	if len(items) == 0 {
		return "[]", nil
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(raw), nil
}

func parse(html string) []data {
	anchors := regexAnchor.FindAllStringIndex(html, -1)

	var results []data
	for i, pos := range anchors {
		anchor := html[pos[0]:pos[1]]

		title := ""
		if m := regexTitle.FindStringSubmatch(anchor); len(m) >= 2 {
			title = extractText(m[1])
		}
		if title == "" {
			continue
		}

		href := ""
		if m := regexHref.FindStringSubmatch(anchor); len(m) >= 2 {
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
		if m := regexSnippet.FindStringSubmatch(html[pos[1]:segEnd]); len(m) >= 2 {
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

func extractURL(str string) string {
	if strings.HasPrefix(str, "http") && !strings.Contains(str, "duckduckgo.com") {
		return str
	}

	parsed, err := url.Parse(str)
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

func filterLinks(ctx context.Context, items []data) []data {
	urls := make([]string, len(items))
	for i, it := range items {
		urls[i] = it.URL
	}
	checks := utils.CheckLinks(ctx, urls)
	filtered := make([]data, 0, len(items))
	pos := 1
	for i, it := range items {
		if checks[i].Status >= 400 {
			continue
		}
		it.URL = checks[i].URL
		it.Position = pos
		pos++
		filtered = append(filtered, it)
	}
	return filtered
}

func extractText(str string) string {
	ddgEntities := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
		"&nbsp;": " ",
	}
	str = regexTag.ReplaceAllString(str, "")

	for entity, char := range ddgEntities {
		str = strings.ReplaceAll(str, entity, char)
	}
	return strings.TrimSpace(str)
}
