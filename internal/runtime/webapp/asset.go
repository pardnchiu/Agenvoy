package webapp

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/page"
)

type asset struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	SHA384 string `json:"sha384"`
}

func SyncAsset(ctx context.Context) error {
	stamp := filepath.Join(filesystem.VendorDir, ".version")
	if current, err := go_pkg_filesystem.ReadText(stamp); err == nil && current == runtime.CurrentVersion {
		return nil
	}

	raw, err := manifest()
	if err != nil {
		return err
	}

	var list []asset
	if err := json.Unmarshal(raw, &list); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "json.Unmarshal", "", err)
	}

	if err := go_pkg_filesystem.CheckDir(filesystem.VendorDir, true); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", "", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	for _, item := range list {
		body, _, err := go_pkg_http.GET[string](ctx, client, item.URL, nil)
		if err != nil {
			return utilsmsg.Wrap(utilsmsg.Upstream, "download", item.Path, err)
		}
		sum := sha512.Sum384([]byte(body))
		if got := "sha384-" + base64.StdEncoding.EncodeToString(sum[:]); got != item.SHA384 {
			return utilsmsg.New(utilsmsg.Upstream, "download "+item.Path, "sha384 mismatch: want %q, got %q", item.SHA384, got)
		}
		if err := go_pkg_filesystem.WriteFile(filepath.Join(filesystem.VendorDir, item.Path), body, 0644); err != nil {
			return utilsmsg.Wrap(utilsmsg.Internal, "write", item.Path, err)
		}
		slog.Debug("webapp asset downloaded",
			slog.String("path", item.Path),
			slog.Int("bytes", len(body)))
	}

	return go_pkg_filesystem.WriteFile(stamp, runtime.CurrentVersion, 0644)
}

func manifest() ([]byte, error) {
	if dir := os.Getenv("AGENVOY_PAGE_DIR"); dir != "" {
		content, err := go_pkg_filesystem.ReadText(filepath.Join(dir, "vendor.json"))
		if err != nil {
			return nil, utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.ReadText", "", err)
		}
		return []byte(content), nil
	}

	raw, err := page.FS.ReadFile("vendor.json")
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "page.FS.ReadFile", "", err)
	}
	return raw, nil
}
