package fetchPage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	go_browser "github.com/pardnchiu/go-browser/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/toolcache"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	skippedExpired    = 12 * time.Hour
	emptySkipExpired  = 72 * time.Hour
	skip4xxExpired    = 90 * 24 * time.Hour
	maxMarkdownLength = 100 << 10
	defaultScroll     = 3
	maxScroll         = 20
)

func currentProfile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "Default"
	}
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Local State")}
	case "linux":
		paths = []string{filepath.Join(home, ".config", "google-chrome", "Local State")}
	default:
		return "Default"
	}
	type localState struct {
		Profile struct {
			LastUsed string `json:"last_used"`
		} `json:"profile"`
	}
	for _, p := range paths {
		ls, err := go_pkg_filesystem.ReadJSON[localState](p)
		if err != nil {
			continue
		}
		if ls.Profile.LastUsed != "" {
			return ls.Profile.LastUsed
		}
	}
	return "Default"
}

func ParseOutputType(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "markdown":
		return go_browser.TypeMarkdown, nil
	case "html":
		return go_browser.TypeHTML, nil
	case "json":
		return go_browser.TypeJSON, nil
	default:
		return 0, utilsmsg.New(utilsmsg.Invalid, "type", "%q: want markdown|html|json", s)
	}
}

func registFetchPage() {
	toolregister.Regist(toolregister.Def{
		Name:        "fetch_page",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     90 * time.Second,
		Description: `[system-default] Fetches one web page in a browser carrying the user's Chrome login, and returns it as markdown, html or json.
Use when a URL is in hand — 讀這篇 / 這個連結寫什麼 / 打開這頁 — for each search or RSS result being cited, and for researching a site page by page until complete.
Before the first page of a site, fetch its /llms.txt; when it lists the page you want, fetch that .md URL instead.
No URL yet → search_web; saving a page or file to disk → download_file; an API endpoint → http_request.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "Full URL including https://.",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "true keeps same-domain links in the output, for following subpages of a docs site or multi-page article.",
					"default":     false,
				},
				"same_session": map[string]any{
					"type":        "boolean",
					"description": "Send the user's Chrome cookies and login state, so login-only pages (social feeds, paywalled news, dashboards) load. false only when the logged-out view is asked for.",
					"default":     true,
				},
				"headless": map[string]any{
					"type":        "boolean",
					"description": "true tries headless first and opens a window only when blocked. false opens a visible window right away: \"open:[url]\" requests, bot checks, JS challenges, pages needing interaction.",
					"default":     true,
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "markdown for reading and summarising; json for structured extraction, comparing items, or feeding another tool; html only when raw DOM is needed.",
					"default":     "markdown",
				},
				"scroll": map[string]any{
					"type":        "integer",
					"description": "Scroll passes to load lazy content, stopping early once the page stops changing: 10-20 for feeds, timelines and long comment threads; 0 when only the top of the page is needed.",
					"minimum":     0,
					"maximum":     maxScroll,
					"default":     defaultScroll,
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "true skips the recent-result cache and re-fetches, when the user asks to refresh or re-check.",
					"default":     false,
				},
			},
			"required": []string{
				"link",
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Link        string `json:"link"`
				KeepLinks   bool   `json:"keep_links"`
				SameSession *bool  `json:"same_session"`
				Headless    *bool  `json:"headless"`
				Type        string `json:"type"`
				Force       bool   `json:"force"`
				Scroll      *int   `json:"scroll"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "link", "required")
			}
			outType, err := ParseOutputType(params.Type)
			if err != nil {
				return "", err
			}
			deliver := func(result string) string {
				if outType != go_browser.TypeMarkdown {
					return result
				}
				return checkRelevance(ctx, e.Objective, link, result)
			}

			if !params.Force {
				if cached, ok := toolcache.FindRecent(e.SessionID, "fetch_page", string(args)); ok {
					return deliver(cached), nil
				}
			}
			sameSession := true
			if params.SameSession != nil {
				sameSession = *params.SameSession
			}
			headless := true
			if params.Headless != nil {
				headless = *params.Headless
			}
			scroll := defaultScroll
			if params.Scroll != nil {
				scroll = min(max(*params.Scroll, 0), maxScroll)
			}

			result, err := handler(ctx, link, params.KeepLinks, sameSession, headless, outType, scroll, nil, params.Force)
			if err != nil {
				return "", err
			}
			toolcache.Store(e.SessionID, go_pkg_utils.UUID(), "fetch_page", string(args), result)
			return deliver(result), nil
		},
	})
}

func handler(ctx context.Context, link string, keepLinks, sameSession, headless bool, outType, scroll int, saveTo *string, force bool) (string, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Invalid, "url.Parse", "", err)
	}
	if err := validateURL(link); err != nil {
		return "", err
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Fragment = ""
	link = parsed.String()

	if hit, _, _ := isSkipped(link); hit && !force {
		return "", utilsmsg.New(utilsmsg.Unavailable, link, "blocked")
	}

	opt := &go_browser.Option{
		MaxLength:   maxMarkdownLength,
		KeepLinks:   keepLinks,
		SameSession: sameSession,
		Visible:     !headless,
		Type:        outType,
		ScrollCount: scroll,
	}
	if scroll == 0 {
		opt.ScrollCount = -1
	}
	if sameSession || !headless {
		opt.Profile = currentProfile()
	}
	result, err := go_browser.Fetch(ctx, link, 30*time.Second, opt)
	if err != nil {
		var fe *go_browser.Error
		if !errors.As(err, &fe) {
			slog.Error("go_browser.Fetch",
				slog.String("url", link),
				slog.String("error", err.Error()))
			return "", utilsmsg.Trace("fetch_page "+link, "", err)
		}
		title := ""
		if result != nil {
			title = result.Title
		}
		addToSkippedMap(link, fe.Status, title)
		return "", utilsmsg.New(utilsmsg.Upstream, link, "fetch failed [%d]", fe.Status)
	}

	var tree []json.RawMessage
	if outType == go_browser.TypeJSON {
		var serialized struct {
			Title string
			Tree  []json.RawMessage
		}
		if err := json.Unmarshal([]byte(result.Content), &serialized); err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Upstream, "json.Unmarshal", link, err)
		}
		result.Title = serialized.Title
		tree = serialized.Tree
	}

	if isPage4xx(result.Title, result.FinalURL) {
		addToSkippedMap(link, 404, result.Title)
		return "", utilsmsg.New(utilsmsg.NotFound, link, "fetch failed [404]")
	}

	var full string
	switch outType {
	case go_browser.TypeMarkdown:
		if strings.TrimSpace(result.Content) == "" {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		if !keepLinks && saveTo == nil {
			result.Content = removeNoise(ctx, link, result.Content)
		}
		full = buildFrontmatter(result)
	case go_browser.TypeJSON:
		if len(tree) == 0 {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		raw, err := json.Marshal(tree)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "tree", err)
		}
		full = string(raw)
	default:
		if strings.TrimSpace(result.Content) == "" {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		full = result.Content
	}

	if saveTo != nil {
		if err := go_pkg_filesystem.CheckDir(filepath.Dir(*saveTo), true); err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", filepath.Dir(*saveTo), err)
		}
		if err := go_pkg_filesystem.WriteFile(*saveTo, full, 0644); err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.WriteFile", *saveTo, err)
		}
		return fmt.Sprintf("Downloaded %d chars to %s", len(full), *saveTo), nil
	}
	return truncateResult(full), nil
}

func SavePage(ctx context.Context, link string, outType int, path string) (string, error) {
	return handler(ctx, link, false, true, true, outType, defaultScroll, &path, false)
}

func validateURL(href string) error {
	if len(href) > 2048 {
		return utilsmsg.New(utilsmsg.Invalid, "url", "too long: max 2048 chars")
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Invalid, "url.Parse", "", err)
	}

	if parsed.User != nil {
		return utilsmsg.New(utilsmsg.Invalid, "url", "must not contain credentials")
	}

	if !strings.Contains(parsed.Hostname(), ".") {
		return utilsmsg.New(utilsmsg.Invalid, "url", "must have a valid hostname")
	}
	return nil
}

func truncateResult(result string) string {
	if len(result) >= maxMarkdownLength {
		result = result[:maxMarkdownLength] + "\n\n[Content truncated due to length...]"
	}
	return fmt.Sprintf("Web page content:\n---\n%s\n---", result)
}

func buildFrontmatter(r *go_browser.Result) string {
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
	sb.WriteString(r.Content)
	return sb.String()
}

func writeField(sb *strings.Builder, key, val string) {
	val = strings.ReplaceAll(val, `"`, `\"`)
	fmt.Fprintf(sb, "%s: \"%s\"\n", key, val)
}
