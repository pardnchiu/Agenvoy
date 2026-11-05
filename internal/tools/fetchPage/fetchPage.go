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

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
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

func parseOutputType(s string) (int, error) {
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
		Description: `[system-default] Fetches one web page in a browser and returns it as markdown, html or json, or saves it to a file.
Use when a URL is in hand — 讀這篇 / 這個連結寫什麼 / 下載網頁 / 存到本地 — for any search or RSS result being cited, and for researching a site, product or library page by page until complete.
Before the first page of a site, fetch its /llms.txt: when it lists the page you want, fetch that listed .md URL instead of the HTML page; no /llms.txt → fetch the page as usual.
No URL yet → search_web; a binary file → download_file; an API endpoint → http_request.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "The full URL of the page to fetch (must include https://).",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "Keep hyperlinks from the same domain. Set true for document research tasks that require recursively following subpages (docs sites, multi-page articles).",
					"default":     false,
				},
				"same_session": map[string]any{
					"type":        "boolean",
					"description": "Reuse the persistent Chrome profile so cookies and login state are sent. Default true, so login-required sites (x.com / twitter / threads / facebook / instagram / linkedin / weibo / xiaohongshu / bloomberg / wsj / ft / dashboards) work transparently. Set false only when explicitly testing the anonymous / logged-out view of a page.",
					"default":     true,
				},
				"headless": map[string]any{
					"type":        "boolean",
					"description": "Default true — attempt headless fetch first, retrying non-headless only on failure. Set false to open a visible browser window immediately, skipping the headless attempt (bot checks, JS challenges, sites requiring visible interaction). Request in the form \"open:[url]\" → always set false.",
					"default":     true,
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "Output format. Default \"markdown\" (best for natural reading and summarisation). Switch to \"json\" when the user asks for structured analysis, data extraction, comparison across multiple items, or when feeding the output to another tool. Switch to \"html\" only when raw DOM is needed for downstream parsing.",
					"default":     "markdown",
				},
				"save": map[string]any{
					"type":        "boolean",
					"description": "Save fetched content to a local file instead of returning it. Default false. Set true when user wants to download/export a page.",
					"default":     false,
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "Target file path when save=true. Absolute path used directly; relative paths resolve against the output directory. Omit for auto-generated filename in the output directory.",
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "Skip the cached-result lookup and re-fetch even if an identical call was made recently. Set true when the user explicitly asks to re-check/refresh a page.",
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
				Save        bool   `json:"save"`
				SaveTo      string `json:"save_to"`
				Force       bool   `json:"force"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "link", "required")
			}
			outType, err := parseOutputType(params.Type)
			if err != nil {
				return "", err
			}
			deliver := func(result string) string {
				if outType != go_browser.TypeMarkdown {
					return result
				}
				return checkRelevance(ctx, e.Objective, link, result)
			}

			if !params.Save && !params.Force {
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

			var saveTo *string
			if params.Save {
				p := strings.TrimSpace(params.SaveTo)
				if p == "" {
					p = defaultDownloadPath(link)
				} else {
					abs, absErr := boundary.Resolve(e.SessionID, filesystem.OutputDir(), p)
					if absErr != nil {
						return "", utilsmsg.Trace("boundary.Resolve", "", absErr)
					}
					p = abs
				}
				saveTo = &p
			}
			result, err := handler(ctx, link, params.KeepLinks, sameSession, headless, outType, saveTo, params.Force)
			if err != nil || saveTo != nil {
				return result, err
			}
			toolcache.Store(e.SessionID, go_pkg_utils.UUID(), "fetch_page", string(args), result)
			return deliver(result), nil
		},
	})
}

func handler(ctx context.Context, link string, keepLinks, sameSession, headless bool, outType int, saveTo *string, force bool) (string, error) {
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
		ScrollCount: defaultScroll,
	}
	if sameSession || !headless {
		opt.Profile = currentProfile()
	}
	result, err := go_browser.Fetch(ctx, link, 30*time.Second, opt)
	if err != nil {
		status := 503
		title := ""
		var fe *go_browser.Error
		if errors.As(err, &fe) {
			status = fe.Status
		} else {
			slog.Debug("go_browser.Fetch",
				slog.String("url", link),
				slog.String("error", err.Error()))
		}
		if result != nil {
			title = result.Title
		}
		addToSkippedMap(link, status, title)
		return "", utilsmsg.New(utilsmsg.Upstream, link, "fetch failed [%d]", status)
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
		if len(result.Tree) == 0 {
			addToSkippedMap(link, 0, result.Title)
			return "", utilsmsg.New(utilsmsg.Upstream, link, "empty content")
		}
		raw, err := json.Marshal(result.Tree)
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
	return filepath.Join(filesystem.OutputDir(), name)
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
