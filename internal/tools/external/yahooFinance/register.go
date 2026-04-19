package yahooFinance

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

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

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "fetch_yahoo_finance",
		ReadOnly: true,
		Description: `
Query Yahoo Finance stock quotes and K-line data, returning current price, intraday high/low, 52-week high/low, volume, and historical OHLCV.
Also send requests to query1 / query2 and use the fastest response.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{
					"type":        "string",
					"description": "Stock symbol, e.g. AAPL, TSLA, ^SPX, 2330.TW",
				},
				"time_interval": map[string]any{
					"type":        "string",
					"description": "K-line interval, available values: 1m / 2m / 5m / 15m / 30m / 60m / 90m / 1h / 1d / 5d / 1wk / 1mo / 3mo, default: 1m",
					"default":     "1m",
					"enum":        timeIntervals,
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Time range, available values: 1d / 5d / 1mo / 3mo / 6mo / 1y / 2y / 5y / 10y / ytd / max, default: 1d",
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

			if params.Symbol == "" {
				params.Symbol = params.Ticker
			}
			if params.Symbol == "" {
				return "", fmt.Errorf("symbol is required")
			}

			if params.TimeInterval == "" || !slices.Contains(timeIntervals, params.TimeInterval) {
				params.TimeInterval = "1m"
			}

			if params.TimeRange == "" || !slices.Contains(timeRanges, params.TimeRange) {
				params.TimeRange = "1d"
			}
			return Fetch(ctx, params.Symbol, params.TimeInterval, params.TimeRange)
		},
	})
}
