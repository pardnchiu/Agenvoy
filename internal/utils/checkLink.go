package utils

import (
	"context"
	"net/http"
	"time"
)

type LinkCheck struct {
	URL    string
	Status int
}

func CheckLinks(ctx context.Context, urls []string) []LinkCheck {
	type entry struct {
		idx    int
		url    string
		status int
	}
	ch := make(chan entry, len(urls))
	for i, u := range urls {
		go func(idx int, link string) {
			resolved, status := resolveLink(ctx, link)
			ch <- entry{idx: idx, url: resolved, status: status}
		}(i, u)
	}
	results := make([]LinkCheck, len(urls))
	for range urls {
		r := <-ch
		results[r.idx] = LinkCheck{URL: r.url, Status: r.status}
	}
	return results
}

func resolveLink(ctx context.Context, link string) (string, int) {
	client := &http.Client{Timeout: 5 * time.Second}
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
