package googleRSS

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	apiPath = "https://news.google.com/rss/search"
)

var timeRanges = []string{
	"1h", "3h", "6h", "12h", "24h", "7d",
}

type responseData struct {
	Channel struct {
		Items []responseItemData `xml:"item"`
	} `xml:"channel"`
}

type responseItemData struct {
	Title       string `xml:"title"       json:"title"`
	Link        string `xml:"link"        json:"link"`
	Description string `xml:"description" json:"description"`
	PubDate     string `xml:"pubDate"     json:"pub_date"`
	Source      struct {
		URL  string `xml:"url,attr"  json:"url"`
		Name string `xml:",chardata" json:"name"`
	} `xml:"source" json:"source"`
}

func Fetch(keyword, timeRange, language string) (string, error) {
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}

	if timeRange == "" {
		timeRange = "7d"
	}

	if language == "" {
		language = "TW:zh-Hant"
	}

	if !slices.Contains(timeRanges, timeRange) {
		return "", fmt.Errorf("invalid interval: %s", timeRange)
	}

	parts := strings.SplitN(language, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid lang format: %s", language)
	}
	geo, lang := parts[0], parts[1]

	q := fmt.Sprintf("%s when:%s", keyword, timeRange)
	requsetPath := fmt.Sprintf("%s?q=%s&hl=%s&gl=%s&ceid=%s",
		apiPath,
		url.QueryEscape(q),
		url.QueryEscape(lang),
		url.QueryEscape(geo),
		url.QueryEscape(language),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	items, err := fetch(ctx, requsetPath)
	if err != nil {
		return "", fmt.Errorf("failed to fetch: %w", err)
	}

	return items, nil
}

func fetch(ctx context.Context, path string) (string, error) {
	data, _, err := utils.GET[responseData](ctx, nil, path, map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Accept":     "application/xml",
	})

	if len(data.Channel.Items) == 0 {
		return "", fmt.Errorf("no result")
	}

	// * remove duplicates and cap at 10
	items := deduplicate(data.Channel.Items)
	if len(items) > 10 {
		items = items[:10]
	}

	out, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("failed to marshal: %w", err)
	}

	return string(out), nil
}

func deduplicate(items []responseItemData) []responseItemData {
	done := make(map[uint64]bool)
	newItems := make([]responseItemData, 0, len(items))

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
