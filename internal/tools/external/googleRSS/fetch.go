package googleRSS

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	go_utils_http "github.com/pardnchiu/go-utils/http"
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

func Fetch(ctx context.Context, keyword, timeRange, language string) (string, error) {
	if timeRange == "" || !slices.Contains(timeRanges, timeRange) {
		timeRange = "7d"
	}

	var geo, lang string
	parts := strings.SplitN(language, ":", 2)
	if language == "" || len(parts) != 2 {
		language = "TW:zh-Hant"
		geo, lang = "TW", "zh-Hant"
	} else {
		geo, lang = parts[0], parts[1]
	}

	query := fmt.Sprintf("%s when:%s", keyword, timeRange)
	requsetPath := fmt.Sprintf("%s?q=%s&hl=%s&gl=%s&ceid=%s",
		path,
		url.QueryEscape(query),
		url.QueryEscape(lang),
		url.QueryEscape(geo),
		url.QueryEscape(language),
	)

	hash := sha256.Sum256([]byte(keyword + "|" + timeRange + "|" + language))
	cacheKey := "rss:" + hex.EncodeToString(hash[:])
	db := store.DB(store.DBToolCache)
	if entry, ok := db.Get(cacheKey); ok {
		return entry.Value, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	items, err := fetch(ctx, requsetPath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch google rss: %w", err)
	}

	if err = db.Set(cacheKey, items, store.SetDefault, store.TTL(ttl)); err != nil {
		slog.Warn("db.Set",
			slog.String("error", err.Error()))
	}

	return items, nil
}

func fetch(ctx context.Context, path string) (string, error) {
	data, status, err := go_utils_http.GET[data](ctx, nil, path, map[string]string{
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
		return "", fmt.Errorf("no result")
	}

	items := deduplicate(data.Channel.Items)
	if len(items) > 10 {
		items = items[:10]
	}

	bytes, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
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
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	s := strings.Join(parts, "")
	hash := uint64(offset64)
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}
