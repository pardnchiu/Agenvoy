package googleRSS

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	path = "https://news.google.com/rss/search"
	ttl  = 300
)

type data struct {
	Channel struct {
		Items []item `xml:"item"`
	} `xml:"channel"`
}

type item struct {
	Title       string `xml:"title"       json:"title"`
	Link        string `xml:"link"        json:"link"`
	Description string `xml:"description" json:"description"`
	PubDate     string `xml:"pubDate"     json:"pub_date"`
	Source      struct {
		URL  string `xml:"url,attr"  json:"url"`
		Name string `xml:",chardata" json:"name"`
	} `xml:"source" json:"source"`
}

func handler(ctx context.Context, keyword, timeRange, ceid, geo, lang string) (string, error) {
	reqPath := fmt.Sprintf("%s?q=%s&hl=%s&gl=%s&ceid=%s",
		path,
		url.QueryEscape(fmt.Sprintf("%s when:%s", keyword, timeRange)),
		url.QueryEscape(lang),
		url.QueryEscape(geo),
		url.QueryEscape(ceid),
	)

	hash := sha256.Sum256([]byte(keyword + "|" + timeRange + "|" + ceid))
	cacheKey := "rss:" + hex.EncodeToString(hash[:])
	db := torii.DB(torii.DBToolCache)
	if entry, ok := db.Get(cacheKey); ok {
		return entry.Value(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	items, err := fetch(ctx, reqPath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch google rss: %w", err)
	}

	if items != "[]" {
		if err = db.Set(cacheKey, items, torii.SetDefault, torii.TTL(ttl)); err != nil {
			slog.Warn("db.Set",
				slog.String("error", err.Error()))
		}
	}

	return items, nil
}

func fetch(ctx context.Context, reqPath string) (string, error) {
	data, status, err := go_pkg_http.GET[data](ctx, nil, reqPath, map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Accept":     "application/xml",
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d", status)
	}
	if len(data.Channel.Items) == 0 {
		return "[]", nil
	}

	items := deduplicate(data.Channel.Items)
	if len(items) > 10 {
		items = items[:10]
	}

	items = resolveLinks(ctx, items)
	if len(items) == 0 {
		return "[]", nil
	}

	raw, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(raw), nil
}

func deduplicate(items []item) []item {
	done := make(map[uint64]bool)
	newItems := make([]item, 0, len(items))

	for _, item := range items {
		key := hash(item.Title, item.Source.Name)

		if !done[key] {
			done[key] = true
			newItems = append(newItems, item)
		}
	}

	return newItems
}

func hash(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		io.WriteString(h, p)
	}
	return h.Sum64()
}

func resolveLinks(ctx context.Context, items []item) []item {
	type result struct {
		idx    int
		url    string
		status int
	}
	ch := make(chan result, len(items))
	for i, it := range items {
		go func(idx int, link string) {
			u, s := resolveLink(ctx, link)
			ch <- result{idx: idx, url: u, status: s}
		}(i, it.Link)
	}
	resolved := make(map[int]result, len(items))
	for range items {
		r := <-ch
		resolved[r.idx] = r
	}
	filtered := make([]item, 0, len(items))
	for i, it := range items {
		r := resolved[i]
		if r.status >= 400 {
			continue
		}
		it.Link = r.url
		filtered = append(filtered, it)
	}
	return filtered
}

func resolveLink(ctx context.Context, link string) (string, int) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return link, 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return link, 0
	}
	resp.Body.Close()
	return resp.Request.URL.String(), resp.StatusCode
}
