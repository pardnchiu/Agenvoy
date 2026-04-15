package yahooFinance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	goutilshttp "github.com/pardnchiu/go-utils/http"
)

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
}

func fetch(ctx context.Context, host, symbol, interval, rangeStr string) (string, error) {
	q := url.Values{}
	q.Set("interval", interval)
	q.Set("range", rangeStr)
	endpoint := fmt.Sprintf("https://%s/v8/finance/chart/%s?%s", host, url.PathEscape(symbol), q.Encode())

	raw, status, err := goutilshttp.GET[string](ctx, httpClient, endpoint, map[string]string{
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Accept":          "application/json",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         "https://finance.yahoo.com",
	})
	if err != nil {
		return "", fmt.Errorf("http.GET: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d from %s", status, host)
	}

	var data any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}

	out, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	return string(out), nil
}
