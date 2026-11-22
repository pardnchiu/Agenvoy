package googleRSS

import (
	"context"
	"slices"
	"strings"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var timeRanges = []string{
	"1h", "3h", "6h", "12h", "24h", "7d",
}

func Search(ctx context.Context, keyword, timeRange, ceid string) (string, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "keyword", "required")
	}

	timeRange = strings.TrimSpace(timeRange)
	if !slices.Contains(timeRanges, timeRange) {
		timeRange = "7d"
	}

	geo, lang := "TW", "zh-Hant"
	ceid = strings.TrimSpace(ceid)
	parts := strings.SplitN(ceid, ":", 2)
	if ceid == "" || len(parts) != 2 {
		ceid = "TW:zh-Hant"
	} else {
		geo, lang = parts[0], parts[1]
	}
	return handler(ctx, keyword, timeRange, ceid, geo, lang)
}
