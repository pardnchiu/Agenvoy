package searchWeb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
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

type TimeRange string

const (
	TimeRange1h    TimeRange = "1h"
	TimeRange3h    TimeRange = "3h"
	TimeRange6h    TimeRange = "6h"
	TimeRange12h   TimeRange = "12h"
	TimeRange1d    TimeRange = "1d"
	TimeRange7d    TimeRange = "7d"
	TimeRangeMonth TimeRange = "1m"
	TimeRangeYear  TimeRange = "1y"
)

func (t TimeRange) valid() bool {
	switch t {
	case TimeRange1h, TimeRange3h, TimeRange6h, TimeRange12h,
		TimeRange1d, TimeRange7d, TimeRangeMonth, TimeRangeYear:
		return true
	}
	return false
}

const cacheExpiry = 5 * time.Minute

func Search(ctx context.Context, query string, timeRange TimeRange) (*SearchOutput, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if timeRange != "" && !timeRange.valid() {
		return nil, fmt.Errorf("invalid time range %q: must be one of 1h, 3h, 6h, 12h, 1d, 7d, 1m, 1y", timeRange)
	}

	hash := sha256.Sum256([]byte(query + "|" + string(timeRange)))
	cacheKey := hex.EncodeToString(hash[:])

	cachedDir := filepath.Join(filesystem.ToolSearchWeb, "cached")
	cleanCache(cachedDir, cacheExpiry)
	cachePath := filepath.Join(cachedDir, cacheKey+".json")
	if info, err := os.Stat(cachePath); err == nil {
		if time.Since(info.ModTime()) < cacheExpiry {
			if data, err := os.ReadFile(cachePath); err == nil {
				var results []ResultData
				if err := json.Unmarshal(data, &results); err == nil {
					return &SearchOutput{Results: results, Cached: true}, nil
				}
			}
		}
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	results, err := fetchDDG(ctx, query, timeRange)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	if err = filesystem.WriteFile(cachePath, string(out), 0644); err != nil {
		slog.Warn("failed to write cache file",
			slog.String("path", err.Error()))
	}

	return &SearchOutput{
		Results:    results,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func cleanCache(dir string, ttl time.Duration) {
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
