// * last modified: 26/11/19
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
	"unicode/utf8"

	go_browser "github.com/pardnchiu/go-browser/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	agenvoyruntime "github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/toolcache"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	skippedExpired   = 12 * time.Hour
	emptySkipExpired = 72 * time.Hour
	skip4xxExpired   = 90 * 24 * time.Hour
	maxFetchLength   = 1 << 20
	challengeWait    = 5 * time.Minute
	fullPageHint     = "full page: download_file(link, render=true), then read_files in parts"
)

func refistFetchPage() {
	toolregister.Regist(toolregister.Def{
		Name:          "fetch_page",
		SystemDefault: true,
		AlwaysAllow:   true,
		Concurrent:    true,
		Timeout:       90*time.Second + challengeWait,
		Description: `Fetches sne web page in a browser carrying the user's Chrome login, and returns it as markdown, html or json.
Use for "read this article", "what does this link say", "open this page", each search or RSS result being cited, and researching a site page by page until complete.
No URL yet → search_web; saving a page or file to disk → download_file; a JSON API endpoint → http_request.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "Full URL, e.g. 'https://example.com/post/1', max 2048 chars; http:// becomes https://, #fragment is dropped. Before the first page of a site, fetch its /llms.txt; when it lists the page you want, fetch that .md URL instead.",
				},
				"output_type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "Output format: markdown (reading and summarising), json (structured extraction, comparing items, feeding another tool), html (raw DOM, only when needed).",
					"default":     "markdown",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "true keeps same-domain links (and skips the boilerplate filter), for following subpages of a docs site or multi-page article.",
					"default":     false,
				},
				"same_session": map[string]any{
					"type":        "boolean",
					"description": "true sends the user's Chrome cookies and login, so login-only pages (social feeds, paywalled news, dashboards) load; false only when the logged-out view is asked for.",
					"default":     true,
				},
				"headless": map[string]any{
					"type":        "boolean",
					"description": "true tries headless first and opens a window only when blocked; false opens a visible window right away, for \"open:[url]\" requests, bot checks, JS challenges and pages needing interaction.",
					"default":     true,
				},
				"scroll": map[string]any{
					"type":        "integer",
					"description": "Scroll passes for lazy content, stopping early once the page stops changing: 10-20 for feeds, timelines and long comment threads; 0 when only the top is needed.",
					"minimum":     0,
					"maximum":     maxScroll,
					"default":     defaultScroll,
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "true skips the 30-minute cache and re-fetches, also retrying a URL marked as failed; use when the user asks to refresh or re-check.",
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
				OutputType  string `json:"output_type"`
				KeepLinks   bool   `json:"keep_links"`
				SameSession *bool  `json:"same_session"`
				Headless    *bool  `json:"headless"`
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

			outType, err := outputType(params.OutputType)
			if err != nil {
				return "", err
			}
			deliver := func(result string) string {
				return deliverPage(ctx, e.Objective, link, outType, params.KeepLinks, result)
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

			var wait time.Duration
			if e.ChallengeEnabled() {
				wait = challengeWait
			}
			result, err := handler(ctx, link, params.KeepLinks, sameSession, headless, outType, scroll, nil, params.Force, wait)
			if errors.Is(err, go_browser.ErrChallengeRequired) {
				enabled, askErr := e.DecideChallenge(func() (bool, error) {
					return askChallenge(ctx, e.SessionID)
				})
				if askErr != nil {
					return "", askErr
				}
				if enabled {
					result, err = handler(ctx, link, params.KeepLinks, sameSession, headless, outType, scroll, nil, true, challengeWait)
				} else {
					result, err = "", challengeDeclined(link)
				}
			}
			if errors.Is(err, go_browser.ErrChallengeClosed) {
				e.ResetChallenge()
				result, err = "", challengeDeclined(link)
			}
			if err != nil {
				return "", err
			}
			toolcache.Store(e.SessionID, go_pkg_utils.UUID(), "fetch_page", string(args), result)
			return deliver(result), nil
		},
	})
}

type challengeText struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Enable  string `json:"enable"`
	Skip    string `json:"skip"`
}

var challengeTexts = loadChallengeTexts()

func loadChallengeTexts() map[string]challengeText {
	var dic map[string]challengeText
	if err := json.Unmarshal(configs.ConfirmFetchChallenge, &dic); err != nil {
		panic(fmt.Sprintf("[Internal] confirm/fetch_challenge.json: %v", err))
	}
	if _, ok := dic["en"]; !ok {
		panic(`[Internal] confirm/fetch_challenge.json: missing "en"`)
	}
	return dic
}

func getChallengeText() challengeText {
	if text, ok := challengeTexts[filesystem.CanonicalReplyLang(filesystem.ConfigReplyLang)]; ok {
		return text
	}
	return challengeTexts["en"]
}

func askChallenge(ctx context.Context, sessionID string) (bool, error) {
	origin := agenttypes.OriginFrom(ctx)
	if origin == "" {
		origin = agenvoyruntime.OriginOf(sessionID)
	}
	if agenttypes.NoPromptFrom(ctx) || agenttypes.AutoModeFrom(ctx) != agenttypes.AutoModeOff || !agenvoyruntime.HasListener(ctx, origin) {
		return false, nil
	}
	text := getChallengeText()
	answers, err := interactive.AskPrompt(ctx, sessionID, []agenvoyruntime.Question{{
		Question: text.Title,
		Detail:   fmt.Sprintf(text.Content, int(challengeWait.Minutes())),
		Options:  []string{text.Enable, text.Skip},
	}})
	if err != nil {
		return false, err
	}
	if len(answers) == 0 {
		return false, nil
	}
	answer, _ := answers[0].(string)
	return answer == text.Enable, nil
}

func challengeDeclined(link string) error {
	addToSkippedMap(link, 403, "")
	return utilsmsg.New(utilsmsg.Upstream, link, "fetch failed [403]")
}

func handler(ctx context.Context, link string, keepLinks, sameSession, headless bool, outType, scroll int, saveTo *string, force bool, wait time.Duration) (string, error) {
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
		KeepLinks:     keepLinks,
		SameSession:   sameSession,
		Visible:       !headless,
		Type:          outType,
		ScrollCount:   scroll,
		ChallengeWait: wait,
	}
	if scroll == 0 {
		opt.ScrollCount = -1
	}
	opt.MaxLength = -1
	if saveTo == nil {
		opt.MaxLength = maxFetchLength + 1
	}
	if sameSession || !headless {
		opt.Profile = currentProfile()
	}

	result, err := go_browser.Fetch(ctx, link, 30*time.Second, opt)
	if errors.Is(err, go_browser.ErrChallengeRequired) || errors.Is(err, go_browser.ErrChallengeClosed) {
		return "", err
	}
	if errors.Is(err, go_browser.ErrChallenge) {
		return "", utilsmsg.New(utilsmsg.Timeout, link, "verification was not completed in the browser window within %s; ask the user to retry and complete it", challengeWait)
	}
	if err != nil {
		var fetchErr *go_browser.Error
		if !errors.As(err, &fetchErr) {
			slog.Error("go_browser.Fetch",
				slog.String("url", link),
				slog.String("error", err.Error()))
			return "", utilsmsg.Trace("fetch_page "+link, "", err)
		}

		title := ""
		if result != nil {
			title = result.Title
		}
		// NOTE: 加入 map 方便後續直接跳過
		addToSkippedMap(link, fetchErr.Status, title)
		return "", utilsmsg.New(utilsmsg.Upstream, link, "fetch failed [%d]", fetchErr.Status)
	}

	if saveTo == nil && outType == go_browser.TypeHTML && len(result.Content) > maxFetchLength {
		return "", utilsmsg.New(utilsmsg.Limit, link, "html content is over the 1 MiB limit; use output_type=markdown, or %s", fullPageHint)
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

	if result.Status >= 400 || isTitleContains4xx(result.Title) {
		addToSkippedMap(link, result.Status, result.Title)
		tag := utilsmsg.NotFound
		if result.Status >= 400 && result.Status != 404 && result.Status != 410 {
			tag = utilsmsg.Upstream
		}
		return "", utilsmsg.New(tag, link, "fetch failed [%d]: %q", result.Status, result.Title)
	}

	var full string
	switch outType {
	case go_browser.TypeMarkdown:
		if strings.TrimSpace(result.Content) == "" {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		if !keepLinks && saveTo == nil && len(result.Content) <= maxFetchLength {
			result.Content = removeNoise(ctx, link, result.Content)
		}
		full = buildFrontmatter(result)
	case go_browser.TypeJSON:
		if len(tree) == 0 {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		var page any = tree
		if saveTo == nil {
			page = jsonPage{Title: result.Title, Tree: tree}
		}
		raw, err := json.Marshal(page)
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
	return full, nil
}

func headBytes(content string, limit int) string {
	if len(content) <= limit {
		return content
	}
	for limit > 0 && !utf8.RuneStart(content[limit]) {
		limit--
	}
	return content[:limit]
}

func isTitleContains4xx(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "404", "403", "not found", "page not found",
		"404 not found", "403 forbidden", "access denied",
		"找不到頁面", "頁面不存在", "此頁面不存在":
		return true
	}
	return false
}

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

func outputType(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "markdown":
		return go_browser.TypeMarkdown, nil
	case "html":
		return go_browser.TypeHTML, nil
	case "json":
		return go_browser.TypeJSON, nil
	default:
		return 0, utilsmsg.New(utilsmsg.Invalid, "output_type", "%q: want markdown|html|json", s)
	}
}

func validateURL(href string) error {
	if len(href) > 2048 {
		return utilsmsg.New(utilsmsg.Invalid, "link", "too long: max 2048 chars")
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Invalid, "url.Parse", "", err)
	}

	if parsed.User != nil {
		return utilsmsg.New(utilsmsg.Invalid, "link", "must not contain credentials")
	}

	if !strings.Contains(parsed.Hostname(), ".") {
		return utilsmsg.New(utilsmsg.Invalid, "link", "must have a valid hostname")
	}
	return nil
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
