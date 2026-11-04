package searchWeb

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/tools/external/searchWeb/googleRSS"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/toolcache"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var timeRanges = []string{"1h", "6h", "24h", "7d", "30d", "1y"}

// * DuckDuckGo takes d/w/m/y, Google News RSS takes 1h..7d — one vocabulary maps onto both
var (
	webWindow = map[string]string{
		"1h": "d", "6h": "d", "24h": "d", "7d": "w", "30d": "m", "1y": "y",
	}
	newsWindow = map[string]string{
		"1h": "1h", "6h": "6h", "24h": "24h", "7d": "7d", "30d": "7d", "1y": "7d",
	}
)

type searchResult struct {
	Web       json.RawMessage `json:"web,omitempty"`
	News      json.RawMessage `json:"news,omitempty"`
	WebError  string          `json:"web_error,omitempty"`
	NewsError string          `json:"news_error,omitempty"`
}

func Register() {
	toolregister.Regist(toolregister.Def{
		Name:        "search_web",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     90 * time.Second,
		Description: `[system-default] Live web lookup — DuckDuckGo results and Google News headlines together, returned as {"web":[...],"news":[...]}.
Use for named entities, post-cutoff facts, versions, prices, 新聞 / 最新消息 / 現在怎麼樣了 / 查一下.
Results are snippets: a result link worth citing → fetch_page. A URL already in hand → fetch_page directly.
Several URLs from one domain on one topic → the most relevant, plus any that carries a figure or date the others lack.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Natural-language keywords, never a URL — 'React 19 release notes', 'TSMC earnings'.",
				},
				"source": map[string]any{
					"type":        "string",
					"enum":        []string{"all", "web", "news"},
					"description": "Which sources to hit. Keep all unless the question is only about one — web for documentation and general facts, news for headlines.",
					"default":     "all",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        timeRanges,
					"description": "Lookback window, applied to both sources. Use 7d for 最近/近期/本週, 30d for 本月. News caps at 7d.",
					"default":     "7d",
				},
				"ceid": map[string]any{
					"type":        "string",
					"description": "News edition (e.g. 'TW:zh-Hant', 'US:en').",
					"default":     "TW:zh-Hant",
				},
				"cdp": map[string]any{
					"type":        "boolean",
					"description": "Force the browser-based web fetch instead of HTTP POST. Slower but bypasses rate-limiting; auto-enabled on HTTP 202.",
					"default":     false,
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "Skip the cached-result lookup and search again. Set true when the user asks to re-check or refresh.",
					"default":     false,
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query     string `json:"query"`
				Source    string `json:"source"`
				TimeRange string `json:"time_range"`
				CEID      string `json:"ceid"`
				CDP       bool   `json:"cdp"`
				Force     bool   `json:"force"`
				Keyword   string `json:"keyword"`
				// avoid small agent like 4.1 be stupid to call with different parameter name
				Q string `json:"q"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			query := firstNonEmpty(params.Query, params.Keyword, params.Q)
			if query == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "query", "required")
			}

			if !params.Force {
				if cached, ok := toolcache.FindRecent(e.SessionID, "search_web", string(args)); ok {
					return keepRelated(ctx, e.Objective, query, cached), nil
				}
			}

			source := strings.TrimSpace(params.Source)
			switch source {
			case "web", "news":
			default:
				source = "all"
			}

			window := strings.TrimSpace(params.TimeRange)
			if _, ok := webWindow[window]; !ok {
				window = "7d"
			}

			var out searchResult
			var wg sync.WaitGroup

			if source != "news" {
				wg.Go(func() {
					raw, err := handler(ctx, query, webWindow[window], params.CDP)
					if err != nil {
						out.WebError = err.Error()
						return
					}
					out.Web = json.RawMessage(raw)
				})
			}
			if source != "web" {
				wg.Go(func() {
					raw, err := googleRSS.Search(ctx, query, newsWindow[window], params.CEID)
					if err != nil {
						out.NewsError = err.Error()
						return
					}
					out.News = json.RawMessage(raw)
				})
			}
			wg.Wait()

			if out.WebError != "" && out.NewsError != "" {
				return "", utilsmsg.New(utilsmsg.Upstream, "search_web", "web: %s; news: %s", out.WebError, out.NewsError)
			}

			raw, err := utils.MarshalPlain(out)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "utils.MarshalPlain", "", err)
			}
			toolcache.Store(e.SessionID, go_pkg_utils.UUID(), "search_web", string(args), string(raw))
			return keepRelated(ctx, e.Objective, query, string(raw)), nil
		},
	})
}

func firstNonEmpty(list ...string) string {
	for _, one := range list {
		if s := strings.TrimSpace(one); s != "" {
			return s
		}
	}
	return ""
}
