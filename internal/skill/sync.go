package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	apiGithubSkills = "https://api.github.com/repos/pardnchiu/agenvoy/contents/extensions/skills"
)

type ghEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

func SyncSkills(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// dir, err := utils.GetConfigDir("skills")
	// if err != nil {
	// 	slog.Error("utils.GetConfigDir",
	// 		slog.String("error", err.Error()))
	// 	return
	// }

	entries, err := listGitHub(ctx, apiGithubSkills)
	if err != nil {
		slog.Error("listGitHub",
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		if entry.Type != "dir" {
			continue
		}

		path := filepath.Join(filesystem.SkillsDir, entry.Name)
		if err := os.Mkdir(path, 0755); err != nil {
			if os.IsExist(err) {
				continue
			}
			slog.Warn("os.Mkdir",
				slog.String("error", err.Error()))
			continue
		}

		if err := downloadDir(ctx, fmt.Sprintf("%s/%s", apiGithubSkills, entry.Name), path); err != nil {
			slog.Warn("downloadDir",
				slog.String("error", err.Error()))
			os.Remove(path)
		}
	}
}

func listGitHub(ctx context.Context, apiURL string) ([]ghEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, apiURL)
	}

	var entries []ghEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("json.Decode: %w", err)
	}
	return entries, nil
}

func downloadDir(ctx context.Context, apiURL, localPath string) error {
	entries, err := listGitHub(ctx, apiURL)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	for _, entry := range entries {
		dest := filepath.Join(localPath, entry.Name)
		switch entry.Type {
		case "dir":
			if err := downloadDir(ctx, fmt.Sprintf("%s/%s", apiURL, entry.Name), dest); err != nil {
				return err
			}
		case "file":
			if entry.DownloadURL == "" {
				continue
			}
			if err := downloadFile(ctx, entry.DownloadURL, dest); err != nil {
				return err
			}
		}
	}
	return nil
}

func downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("http.NewRequest: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("os.Create: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}
	return nil
}
