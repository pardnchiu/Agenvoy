package searchWeb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
)

type ResultData struct {
	Position    int    `json:"position"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type SearchOutput struct {
	Results    []ResultData `json:"results"`
	DurationMs int64        `json:"duration_ms"`
	Cached     bool         `json:"cached,omitempty"`
}

var timeRanges = []string{"1d", "7d", "1m", "1y"}

type TimeRange string

const (
	TimeRange1d    TimeRange = "1d"
	TimeRange7d    TimeRange = "7d"
	TimeRangeMonth TimeRange = "1m"
	TimeRangeYear  TimeRange = "1y"
)

func (t TimeRange) valid() bool {
	return slices.Contains(timeRanges, string(t))
}

const cacheTTLSeconds = 300

func Search(ctx context.Context, query string, timeRange TimeRange) (*SearchOutput, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if timeRange != "" && !timeRange.valid() {
		return nil, fmt.Errorf("invalid time range %q: must be one of %s", timeRange, strings.Join(timeRanges, ", "))
	}

	hash := sha256.Sum256([]byte(query + "|" + string(timeRange)))
	cacheKey := "search:" + hex.EncodeToString(hash[:])

	db := store.DB(store.DBToolCache)
	if entry, ok := db.Get(cacheKey); ok {
		var results []ResultData
		if err := json.Unmarshal([]byte(entry.Value), &results); err == nil {
			return &SearchOutput{Results: results, Cached: true}, nil
		}
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	results, err := fetch(ctx, query, timeRange)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	if err = db.Set(cacheKey, string(out), store.SetDefault, store.TTL(cacheTTLSeconds)); err != nil {
		slog.Warn("store.Set search cache",
			slog.String("error", err.Error()))
	}

	return &SearchOutput{
		Results:    results,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
