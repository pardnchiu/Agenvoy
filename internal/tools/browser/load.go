package browser

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
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

func Load(href string) (string, error) {
	if href == "" {
		return "", fmt.Errorf("href is required")
	}

	cached5xx := filepath.Join(filesystem.ToolsDir, "browser", "5xx")
	// if dir, err := utils.GetConfigDir("tools", "browser", "5xx"); err == nil {
	// 	clean(dir.Home, skippedExpired)
	// }
	clean(cached5xx, skippedExpired)

	if isSkipped(href) {
		return skippedMessage(href), nil
	}

	hash := sha256.Sum256([]byte(href + "|text"))
	cacheKey := hex.EncodeToString(hash[:])
	cached := filepath.Join(filesystem.ToolsDir, "browser", "cached")
	// configDir, err := utils.GetConfigDir("tools", "browser", "cached")
	// if err != nil {
	// 	return "", fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	clean(cached, cacheExpired)
	cachePath := filepath.Join(cached, cacheKey+".md")
	if _, err := os.Stat(cachePath); err == nil {
		if cached, err := os.ReadFile(cachePath); err == nil {
			return string(cached), nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := load(ctx, href)
	if err != nil {
		// * use 503 as default, if over 10 sec no data, tag as timeout
		status := 503
		var fetchErr *FetchError
		if errors.As(err, &fetchErr) {
			status = fetchErr.Status
		}
		addToSkippedMap(href, status)
		// * 4xx and 5xx, just tag skipped, not error
		return skippedMessage(href), nil
	}

	body := data.Markdown
	if idx := strings.Index(body, "---\n\n"); idx != -1 {
		body = strings.TrimSpace(body[idx+5:])
	}
	if body == "" {
		addToSkippedMap(href, 0)
		// * empty data, same as 4xx, just tag skipped
		return skippedMessage(href), nil
	}

	if err = filesystem.WriteFile(cachePath, data.Markdown, 0644); err != nil {
		slog.Warn("utils.WriteFile",
			slog.String("error", err.Error()))
	}
	return data.Markdown, nil
}

func load(ctx context.Context, href string) (*HTMLParser, error) {
	browser, err := newBrowser()
	if err != nil {
		return nil, err
	}
	defer browser.MustClose()

	page, err := fetch(ctx, browser, href)
	if err != nil {
		return nil, err
	}
	defer page.MustClose()

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("page.HTML: %w", err)
	}

	result, err := extract(href, html)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	return result, nil
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

func newBrowser() (*rod.Browser, error) {
	newLauncher := launcher.New().
		Headless(!hasDisplay()).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars", "").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("window-size", "1280,960").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

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
