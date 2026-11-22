package runtime

import (
	"context"
	"net/http"
	"strings"
	"time"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const LATEST_RELEASE_URL = "https://github.com/pardnchiu/Agenvoy/releases/latest/download/x"

var releaseClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func LatestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, LATEST_RELEASE_URL, nil)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "http.NewRequestWithContext", "", err)
	}
	resp, err := releaseClient.Do(req)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Unavailable, "resolve latest release", "", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return "", utilsmsg.New(utilsmsg.Upstream, "resolve latest release", "github http %d", resp.StatusCode)
	}
	parts := strings.Split(strings.Trim(resp.Header.Get("Location"), "/"), "/")
	if len(parts) < 3 || parts[len(parts)-3] != "download" || parts[len(parts)-2] == "" {
		return "", utilsmsg.New(utilsmsg.Upstream, "resolve latest release", "unexpected redirect %q", resp.Header.Get("Location"))
	}
	return parts[len(parts)-2], nil
}
