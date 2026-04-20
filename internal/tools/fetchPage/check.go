package fetchPage

import "strings"

func isPage4xx(title, link string) bool {
	if isTitleContains4xx(title) || isLinkContains4xx(link) {
		return true
	}
	return false
}

func isLinkContains4xx(link string) bool {
	if link == "" {
		return false
	}

	n := len(link)
	for i := 0; i < n; {
		if link[i] < '0' || link[i] > '9' {
			i++
			continue
		}
		j := i
		for j < n && link[j] >= '0' && link[j] <= '9' {
			j++
		}
		switch link[i:j] {
		case "404", "403", "410":
			return true
		}
		i = j
	}
	return false
}

func isTitleContains4xx(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "404", "403", "not found", "page not found",
		"404 not found", "403 forbidden", "access denied",
		"找不到頁面", "頁面不存在", "此頁面不存在":
		return true
	}
	return false
}
