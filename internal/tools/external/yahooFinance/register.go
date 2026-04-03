package yahooFinance

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "fetch_yahoo_finance",
		ReadOnly: true,
		Description: "查詢 Yahoo Finance 股票行情與 K 線資料，返回現價、當日高低、52 週高低、成交量及歷史 OHLCV。" +
			"同時對 query1 / query2 發送請求，取最快回應。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{
					"type":        "string",
					"description": "股票代碼，例如 AAPL、TSLA、^SPX、2330.TW",
				},
				"interval": map[string]any{
					"type":        "string",
					"description": "K 線週期，預設 1m",
					"default":     "1m",
					"enum":        []string{"1m", "2m", "5m", "15m", "30m", "60m", "90m", "1h", "1d", "5d", "1wk", "1mo", "3mo"},
				},
				"range": map[string]any{
					"type":        "string",
					"description": "查詢時間範圍，預設 1d",
					"default":     "1d",
					"enum":        []string{"1d", "5d", "1mo", "3mo", "6mo", "1y", "2y", "5y", "10y", "ytd", "max"},
				},
			},
			"required": []string{"symbol"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Symbol   string `json:"symbol"`
				Ticker   string `json:"ticker"`
				Interval string `json:"interval"`
				Range    string `json:"range"`
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
			if params.Interval == "" {
				params.Interval = "1m"
			}
			if params.Range == "" {
				params.Range = "1d"
			}

			type result struct {
				val string
				err error
			}

			ch := make(chan result, 2)

			for _, host := range []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"} {
				go func(h string) {
					val, err := fetch(ctx, h, params.Symbol, params.Interval, params.Range)
					ch <- result{val, err}
				}(host)
			}

			winCh := make(chan result, 1)
			go func() {
				var once sync.Once
				var lastErr error
				for i := 0; i < 2; i++ {
					r := <-ch
					if r.err == nil {
						once.Do(func() { winCh <- r })
					} else {
						lastErr = r.err
					}
				}
				once.Do(func() { winCh <- result{err: lastErr} })
			}()

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case r := <-winCh:
				return r.val, r.err
			}
		},
	})
}
