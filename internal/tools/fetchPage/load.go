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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

//go:embed embed/stealth.js
var stealthJS string

//go:embed embed/listener.js
var listenerJS string

//go:embed embed/skipped.md
var skippedPrompt string

type FetchError struct {
	Status int
	Href   string
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("http %d: %s", e.Status, e.Href)
}

const (
	networkIdleTimeout = 5 * time.Second
	cacheExpired       = 1 * time.Hour
	skippedExpired     = 12 * time.Hour
	fetchTimeout       = 30 * time.Second
	maxMarkdownLength  = 100 << 10
)

func skippedMessage(href string) string {
	content, err := template.New("skipped").Parse(skippedPrompt)
	if err != nil {
		return href
	}
	var sb strings.Builder
	if err := content.Execute(&sb, struct{ Href string }{href}); err != nil {
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

	cached5xx := filepath.Join(filesystem.ToolFetchPage, "5xx")
	clean(cached5xx, skippedExpired)

	if isSkipped(href) {
		return skippedMessage(href), nil
	}

	cacheVariant := "|text"
	if keepLinks {
		cacheVariant = "|links"
	}
	hash := sha256.Sum256([]byte(href + cacheVariant))
	cacheKey := hex.EncodeToString(hash[:])
	cached := filepath.Join(filesystem.ToolFetchPage, "cached")

	clean(cached, cacheExpired)
	cachePath := filepath.Join(cached, cacheKey+".md")
	if _, err := os.Stat(cachePath); err == nil {
		if b, err := os.ReadFile(cachePath); err == nil {
			return formatForAgent(truncate(string(b))), nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	data, err := fetchAndParse(ctx, href, keepLinks)
	if err != nil {
		status := 503
		var fetchErr *FetchError
		if errors.As(err, &fetchErr) {
			status = fetchErr.Status
		}
		addToSkippedMap(href, status)
		return skippedMessage(href), nil
	}

	if detect4xx(data.Title) {
		addToSkippedMap(href, 404)
		return skippedMessage(href), nil
	}

	body := data.Markdown
	if idx := strings.Index(body, "---\n\n"); idx != -1 {
		body = strings.TrimSpace(body[idx+5:])
	}
	if body == "" {
		addToSkippedMap(href, 0)
		return skippedMessage(href), nil
	}

	if err = filesystem.WriteFile(cachePath, data.Markdown, 0644); err != nil {
		slog.Warn("utils.WriteFile",
			slog.String("error", err.Error()))
	}
	return formatForAgent(truncate(data.Markdown)), nil
}

func fetchAndParse(ctx context.Context, href string, keepLinks bool) (*HTMLParser, error) {
	browser, err := newBrowser()
	if err != nil {
		return nil, err
	}
	defer func() { _ = browser.Close() }()

	page, err := fetch(ctx, browser, href)
	if err != nil {
		return nil, err
	}
	defer func() { _ = page.Close() }()

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("page.HTML: %w", err)
	}

	result, err := extract(href, html, keepLinks)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	return result, nil
}

func urlContains404(finalURL string) bool {
	u, err := url.Parse(finalURL)
	if err != nil {
		return false
	}
	for param := range strings.SplitSeq(u.Path, "/") {
		if param == "404" || param == "403" {
			return true
		}
	}
	for key := range u.Query() {
		val := u.Query().Get(key)
		if val == "404" || val == "403" {
			return true
		}
	}
	return false
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

func fetch(ctx context.Context, browser *rod.Browser, href string) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("browser.Page: %w", err)
	}

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1280,
		Height:            960,
		DeviceScaleFactor: 1,
	}); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("page.SetViewport: %w", err)
	}

	if _, err := page.EvalOnNewDocument(stealthJS); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("page.EvalOnNewDocument: %w", err)
	}

	if err := page.Context(ctx).Navigate(href); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf(" page.Navigate %s: %w", href, err)
	}

	if err := page.Context(ctx).WaitLoad(); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("page.WaitLoad: %w", err)
	}

	if info, err := page.Info(); err == nil && urlContains404(info.URL) {
		_ = page.Close()
		return nil, &FetchError{Status: 404, Href: href}
	}

	if status, err := page.Eval(`() => { const e = performance.getEntriesByType("navigation")[0]; return e ? e.responseStatus : 0 }`); err == nil {
		if code := status.Value.Int(); code >= 400 {
			_ = page.Close()
			return nil, &FetchError{
				Status: code,
				Href:   href,
			}
		}
	}

	_ = page.WaitIdle(networkIdleTimeout)

	stableCtx, stableCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stableCancel()

	// * wait 3 sec for page being rendered
	_, _ = page.Context(stableCtx).Eval(listenerJS)

	return page, nil
}

func hasDisplay() bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func chromePath() string {
	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	case "windows":
		for _, p := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func newBrowser() (*rod.Browser, error) {
	newLauncher := launcher.New().
		Headless(!hasDisplay()).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars", "").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("window-size", "1280,960").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	if chromePath := chromePath(); chromePath != "" {
		newLauncher = newLauncher.Bin(chromePath)
	}

	url, err := newLauncher.Launch()
	if err != nil {
		return nil, fmt.Errorf("launcher.Launch: %w", err)
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("browser.Connect: %w", err)
	}
	return browser, nil
}

func clean(dir string, ttl time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > ttl {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
