package yahooFinance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var (
	timeIntervals = []string{
		"1m", "2m", "5m", "15m", "30m", "60m", "90m", "1h", "1d", "5d", "1wk", "1mo", "3mo",
	}
	timeRanges = []string{
		"1d", "5d", "1mo", "3mo", "6mo", "1y", "2y", "5y", "10y", "ytd", "max",
	}
)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "fetch_yahoo_finance",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Query Yahoo Finance quotes and K-line.
Equity prices, indices, intraday / historical OHLCV.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{
					"type":        "string",
					"description": "Symbol (e.g. 'AAPL', 'TSLA', '^SPX', '2330.TW').",
				},
				"time_interval": map[string]any{
					"type":        "string",
					"description": "Candle interval.",
					"default":     "1m",
					"enum":        timeIntervals,
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Lookback window.",
					"default":     "1d",
					"enum":        timeRanges,
				},
			},
			"required": []string{
				"symbol",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Symbol       string `json:"symbol"`
				TimeInterval string `json:"time_interval"`
				TimeRange    string `json:"time_range"`
				// avoid small agent like 4.1 be stupid to call with different parameter name
				Ticker string `json:"ticker"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			symbol := strings.TrimSpace(params.Symbol)
			if symbol == "" {
				symbol = strings.TrimSpace(params.Ticker)
			}
			if symbol == "" {
				return "", fmt.Errorf("symbol is required")
			}

			timeInterval := strings.TrimSpace(params.TimeInterval)
			if timeInterval != "" && !slices.Contains(timeIntervals, params.TimeInterval) {
				slog.Warn("invalid time_interval, fallback to '1m'")
				timeInterval = "1m"
			}

			timeRange := strings.TrimSpace(params.TimeRange)
			if timeRange != "" && !slices.Contains(timeRanges, params.TimeRange) {
				slog.Warn("invalid time_range, fallback to '1d'")
				timeRange = "1d"
			}
			return handler(ctx, symbol, timeInterval, timeRange)
		},
	})
}
