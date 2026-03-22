package browser

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type HTMLParser struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	PublishedAt string `json:"published_at"`
	Excerpt     string `json:"excerpt"`
	Markdown    string `json:"markdown"`
}

func extract(href, content string, keepLinks bool) (*HTMLParser, error) {
	parsedUrl, err := url.Parse(href)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %w", err)
	}

	article, err := readability.FromReader(strings.NewReader(content), parsedUrl)
	if err != nil {
		return nil, fmt.Errorf("readability.FromReader: %w", err)
	}

	parser := &HTMLParser{
		URL:     href,
		Title:   article.Title,
		Author:  article.Byline,
		Excerpt: article.Excerpt,
	}
	if article.PublishedTime != nil {
		parser.PublishedAt = article.PublishedTime.Format(time.RFC3339)
	}

	newContent := strings.TrimSpace(article.Content)
	if newContent == "" {
		newContent = content
	}

	parser.Markdown, err = transToMarkdown(newContent, href, true, keepLinks)
	if err != nil {
		return nil, fmt.Errorf("transToMarkdown: %w", err)
	}
	parser.Markdown = fullContent(parser)

	return parser, nil
}

func fullContent(parser *HTMLParser) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	writeField(&sb, "title", parser.Title)
	writeField(&sb, "url", parser.URL)
	if parser.Author != "" {
		writeField(&sb, "author", parser.Author)
	}
	if parser.PublishedAt != "" && parser.PublishedAt != "0001-01-01T00:00:00Z" {
		writeField(&sb, "published_at", parser.PublishedAt)
	}
	if parser.Excerpt != "" {
		writeField(&sb, "excerpt", parser.Excerpt)
	}
	sb.WriteString("---\n")
	sb.WriteString(parser.Markdown)
	return sb.String()
}

func writeField(sb *strings.Builder, key, val string) {
	val = strings.ReplaceAll(val, `"`, `\"`)
	fmt.Fprintf(sb, "%s: \"%s\"\n", key, val)
}
