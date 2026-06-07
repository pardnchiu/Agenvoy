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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	go_browser "github.com/pardnchiu/go-browser"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	cacheExpired      = 1 * time.Hour
	skippedExpired    = 12 * time.Hour
	emptySkipExpired  = 72 * time.Hour
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
		return 0, fmt.Errorf("invalid type %q: want markdown|html|json", s)
	}
}

func registFetchPage() {
	toolRegister.Regist(toolRegister.Def{

		Name:        "fetch_page",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "[system-default] Fetch a web page and return content (markdown/html/json). URL given → always this tool, never search_web. same_session=true for login-required sites. Mandatory on search/RSS result links for research tasks or when citing sources. Document research: no request limit — fetch page by page until complete. Set save=true to download the page to a local file (\"下載網頁\", \"存到本地\", \"寫成 md\"); omit save_to for auto-save to ~/Downloads.",
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
					"description": "Reuse the persistent Chrome profile so cookies and login state are sent. Default true — cookies always sent so login-required sites (x.com / twitter / threads / facebook / instagram / linkedin / weibo / xiaohongshu / bloomberg / wsj / ft / dashboards) work transparently. Set false only when explicitly testing the anonymous / logged-out view of a page.",
					"default":     true,
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "Output format. Default \"markdown\" (best for natural reading and summarisation). Switch to \"json\" when the user asks for structured analysis, data extraction, comparison across multiple items, or when feeding the output to another tool. Switch to \"html\" only when raw DOM is needed for downstream parsing.",
					"default":     "markdown",
				},
				"cache": map[string]any{
					"type":        "boolean",
					"description": "Whether to write the fetched result into the local cache. Default true. Set false for one-off pages that won't be revisited (search result URLs, throwaway tokens in querystring) so the cache stays useful.",
					"default":     true,
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "Ignore any existing cache entry and refetch live. Default false. Set true when the user explicitly asks for the latest version (\"重新抓\" / \"最新\" / \"refresh\") or when the previously cached content is known stale.",
					"default":     false,
				},
				"save": map[string]any{
					"type":        "boolean",
					"description": "Save fetched content to a local file instead of returning it. Default false. Set true when user wants to download/export a page.",
					"default":     false,
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "Target file path when save=true. Absolute path used directly; relative paths resolve against ~/Downloads (preferred if exists) or ~/.config/agenvoy/download/. Omit for auto-generated filename.",
				},
			},
			"required": []string{
				"link",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Link        string `json:"link"`
				KeepLinks   bool   `json:"keep_links"`
				SameSession *bool  `json:"same_session"`
				Type        string `json:"type"`
				Cache       *bool  `json:"cache"`
				Force       bool   `json:"force"`
				Save        bool   `json:"save"`
				SaveTo      string `json:"save_to"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", fmt.Errorf("link is required")
			}
			outType, err := parseOutputType(params.Type)
			if err != nil {
				return "", err
			}
			useCache := true
			if params.Cache != nil {
				useCache = *params.Cache
			}
			sameSession := true
			if params.SameSession != nil {
				sameSession = *params.SameSession
			}

			var saveTo *string
			if params.Save {
				p := strings.TrimSpace(params.SaveTo)
				if p == "" {
					p = defaultDownloadPath(link)
				} else {
					abs, absErr := go_pkg_filesystem.AbsPath(filesystem.DownloadDir, p, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
					if absErr != nil {
						return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", absErr)
					}
					p = abs
				}
				saveTo = &p
			}
			return handler(ctx, link, params.KeepLinks, sameSession, outType, useCache, params.Force, saveTo)
		},
	})
}

func handler(ctx context.Context, link string, keepLinks, sameSession bool, outType int, useCache, force bool, saveTo *string) (string, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("url.Parse: %w", err)
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

	if hit, _, _ := isSkipped(link); hit {
		return "", fmt.Errorf("%s blocked", link)
	}

	cacheVariant := fmt.Sprintf("|type=%d|links=%t|session=%t", outType, keepLinks, sameSession)
	hash := sha256.Sum256([]byte(link + cacheVariant))
	cacheKey := "page:" + hex.EncodeToString(hash[:])
	db := torii.DB(torii.DBToolCache)
	var full string
	cacheHit := false
	if !force {
		if entry, ok := db.Get(cacheKey); ok {
			full = entry.Value()
			cacheHit = true
		}
	}
	if cacheHit {
		if saveTo == nil {
			return truncateResult(full), nil
		}
	} else {
		opt := &go_browser.Option{
			MaxLength:   maxMarkdownLength,
			KeepLinks:   keepLinks,
			SameSession: sameSession,
			Type:        outType,
			ScrollCount: defaultScroll,
		}
		if sameSession {
			opt.Profile = currentProfile()
		}
		result, err := go_browser.Fetch(ctx, link, 30*time.Second, opt)
		if err != nil {
			status := 503
			title := ""
			var fe *go_browser.Error
			if errors.As(err, &fe) {
				status = fe.Status
			}
			if result != nil {
				title = result.Title
			}
			addToSkippedMap(link, status, title)
			return "", fmt.Errorf("%s fetch failed [%d]", link, status)
		}

		if isPage4xx(result.Title, result.FinalURL) {
			addToSkippedMap(link, 404, result.Title)
			return "", fmt.Errorf("%s fetch failed [404]", link)
		}

		switch outType {
		case go_browser.TypeMarkdown:
			if strings.TrimSpace(result.Content) == "" {
				addToSkippedMap(link, 0, result.Title)
				return "", fmt.Errorf("%s empty content", link)
			}
			full = buildFrontmatter(result)
		case go_browser.TypeJSON:
			if len(result.Tree) == 0 {
				addToSkippedMap(link, 0, result.Title)
				return "", fmt.Errorf("%s empty content", link)
			}
			raw, err := json.Marshal(result.Tree)
			if err != nil {
				return "", fmt.Errorf("json.Marshal tree: %w", err)
			}
			full = string(raw)
		default:
			if strings.TrimSpace(result.Content) == "" {
				addToSkippedMap(link, 0, result.Title)
				return "", fmt.Errorf("%s empty content", link)
			}
			full = result.Content
		}
		if useCache {
			if err = db.Set(cacheKey, full, torii.SetDefault, torii.TTL(int64(cacheExpired.Seconds()))); err != nil {
				slog.Warn("db.Set",
					slog.String("error", err.Error()))
			}
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
