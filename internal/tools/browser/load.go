package browser

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

//go:embed embed/stealth.js
var stealthJS string

//go:embed embed/listener.js
var listenerJS string

const (
	networkIdleTimeout = 5 * time.Second
	cacheExpiry        = 1 * time.Hour
)

type Data struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

func Load(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	hash := sha256.Sum256([]byte(url))
	cacheKey := hex.EncodeToString(hash[:])

	configDir, err := utils.GetConfigDir("tools", "browser", "cached")

	clean(configDir.Home, cacheExpiry)
	cachePath := filepath.Join(configDir.Home, cacheKey+".md")

	if info, err := os.Stat(cachePath); err == nil {
		if time.Since(info.ModTime()) < cacheExpiry {
			cached, err := os.ReadFile(cachePath)
			if err == nil {
				return string(cached), nil
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	content, err := load(ctx, url)
	if err != nil {
		return "", err
	}
	result := content.Markdown

	// * if wrote err, then skip
	_ = os.WriteFile(cachePath, []byte(result), 0644)

	return result, nil
}

func load(ctx context.Context, url string) (*Data, error) {
	browser, err := newBrowser()
	if err != nil {
		return nil, err
	}
	defer browser.MustClose()

	page, err := fetch(ctx, browser, url)
	if err != nil {
		return nil, err
	}
	defer page.MustClose()

	result := &Data{URL: url}

	if el, err := page.Element("title"); err == nil {
		result.Title, _ = el.Text()
	}

	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("page.HTML: %w", err)
	}

	result.Markdown, err = extract(html, result.Title, url)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	return result, nil
}

func hasDisplay() bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func newBrowser() (*rod.Browser, error) {
	newLancher := launcher.New().
		Headless(!hasDisplay()).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars", "").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Set("window-size", "1920,1080").
		Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	wsURL, err := newLancher.Launch()
	if err != nil {
		return nil, fmt.Errorf("newLancher.Launch: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("browser.Connect: %w", err)
	}
	return browser, nil
}

func fetch(ctx context.Context, browser *rod.Browser, url string) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("browser.Page: %w", err)
	}

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1920,
		Height:            1080,
		DeviceScaleFactor: 1,
	}); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("page.SetViewport: %w", err)
	}

	if _, err := page.EvalOnNewDocument(stealthJS); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("page.EvalOnNewDocument: %w", err)
	}

	// * open page
	if err := page.Context(ctx).Navigate(url); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("Navigate %s: %w", url, err)
	}

	// * wait loading stop
	// page start
	if err := page.Context(ctx).WaitLoad(); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("WaitLoad: %w", err)
	}

	// * check HTTP status via navigation timing API
	if status, err := page.Eval(`() => { const e = performance.getEntriesByType("navigation")[0]; return e ? e.responseStatus : 0 }`); err == nil {
		if code := status.Value.Int(); code >= 400 {
			_ = page.Close()
			return nil, fmt.Errorf("HTTP %d: %s", code, url)
		}
	}
	// page done
	_ = page.WaitIdle(networkIdleTimeout)

	// * wait 4s after onload for dynamic content to settle
	stableCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := page.Context(stableCtx).Eval(listenerJS); err != nil {
		_ = page.Close()
		return nil, fmt.Errorf("listenerJS: %w", err)
	}

	return page, nil
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
