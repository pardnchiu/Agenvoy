package tui

import (
	"context"
	"net/http"
	"strings"
	"time"

	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const projectWorker = "https://guthub-agenvoy.pardn.workers.dev/"

type released struct {
	tag string
}

type projectData struct {
	Releases []struct {
		TagName string `json:"tag_name"`
	} `json:"releases"`
}

func fetchProjectRelease(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data, status, err := go_pkg_http.GET[projectData](ctx, nil, projectWorker, nil)
	if err != nil || status != http.StatusOK {
		return
	}
	if len(data.Releases) == 0 {
		return
	}

	tag := strings.TrimSpace(data.Releases[0].TagName)
	if tag == "" {
		return
	}
	send(released{tag: tag})
}
