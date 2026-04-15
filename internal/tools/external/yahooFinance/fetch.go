package yahooFinance

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
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	go_utils_http "github.com/pardnchiu/go-utils/http"
)

const cacheTTL = 60

func Fetch(ctx context.Context, symbol, timeInterval, timeRange string) (string, error) {
	if timeInterval == "" || !slices.Contains(timeIntervals, timeInterval) {
		timeInterval = "1m"
	}

	if timeRange == "" || !slices.Contains(timeRanges, timeRange) {
		timeRange = "1d"
	}

	hash := sha256.Sum256([]byte(symbol + "|" + timeInterval + "|" + timeRange))
	cacheKey := "yahoo:" + hex.EncodeToString(hash[:])
	db := store.DB(store.DBToolCache)
	if entry, ok := db.Get(cacheKey); ok {
		return entry.Value, nil
	}

	type result struct {
		data string
		err  error
	}

	ch := make(chan result, 2)

	for _, host := range []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"} {
		go func(h string) {
			data, err := fetch(ctx, h, symbol, timeInterval, timeRange)
			if err != nil {
				err = fmt.Errorf("failed to fetch yahoo finance: %w", err)
			}
			ch <- result{data, err}
		}(host)
	}

	winCh := make(chan result, 1)
	go func() {
		var firstErr result
		for range 2 {
			r := <-ch
			if r.err == nil {
				winCh <- r
				return
			}
			if firstErr.err == nil {
				firstErr = r
			}
		}
		winCh <- firstErr
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()

	case r := <-winCh:
		if r.err == nil && r.data != "" {
			if err := db.Set(cacheKey, r.data, store.SetDefault, store.TTL(cacheTTL)); err != nil {
				slog.Warn("db.Set",
					slog.String("error", err.Error()))
			}
		}
		return r.data, r.err
	}
}

func fetch(ctx context.Context, host, symbol, interval, rangeStr string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	q := url.Values{}
	q.Set("interval", interval)
	q.Set("range", rangeStr)
	endpoint := fmt.Sprintf("https://%s/v8/finance/chart/%s?%s", host, url.PathEscape(symbol), q.Encode())

	data, status, err := go_utils_http.GET[any](ctx, nil, endpoint, map[string]string{
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Accept":          "application/json",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         "https://finance.yahoo.com",
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d", status)
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
