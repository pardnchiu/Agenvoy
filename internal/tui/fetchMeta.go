package tui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Commit struct {
	Date    string
	Message string
}

const (
	metaURL = "https://guthub-agenvoy.pardn.workers.dev/"
)

var (
	metaMu          sync.RWMutex
	metaDescription = "..."
	metaCommits     []Commit
	metaRelease     string
)

func fetchMeta() {
	resp, err := http.Get(metaURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		Repo struct {
			Description string `json:"description"`
		} `json:"repo"`
		Commits []struct {
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		} `json:"commits"`
		Releases []struct {
			TagName string `json:"tag_name"`
		} `json:"releases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	metaMu.Lock()
	if result.Repo.Description != "" {
		metaDescription = result.Repo.Description
	}

	if len(result.Releases) > 0 {
		metaRelease = result.Releases[0].TagName
	}

	newCommits := make([]Commit, 0, 3)
	for _, c := range result.Commits[:3] {
		message := c.Commit.Message
		if i := strings.Index(message, "\n"); i != -1 {
			message = message[:i]
		}

		var date string
		if t, err := time.Parse(time.RFC3339, c.Commit.Author.Date); err == nil {
			date = t.Local().Format("2006-01-02 15:04")
		}
		newCommits = append(newCommits, Commit{
			Date:    date,
			Message: message,
		})
	}
	metaCommits = newCommits
	metaMu.Unlock()

	if app != nil && currentPath == "" {
		go app.QueueUpdateDraw(func() {
			contentView.SetText(setDefault())
		})
	}
}
