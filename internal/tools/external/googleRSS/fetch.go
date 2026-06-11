package googleRSS

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const path = "https://news.google.com/rss/search"

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

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return fetch(ctx, reqPath)
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
	urls := make([]string, len(items))
	for i, it := range items {
		urls[i] = it.Link
	}
	checks := utils.CheckLinks(ctx, urls)
	filtered := make([]item, 0, len(items))
	for i, it := range items {
		if checks[i].Status >= 400 {
			continue
		}
		it.Link = checks[i].URL
		filtered = append(filtered, it)
	}
	return filtered
}
