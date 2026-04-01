package searchTools

import (
	"regexp"
	"sort"
	"strings"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type result struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func searchByKeyword(query string, tools []toolTypes.Tool, maxResults int) []result {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	rawTerms := strings.Fields(queryLower)

	var required, optional []string
	for _, t := range rawTerms {
		if strings.HasPrefix(t, "+") && len(t) > 1 {
			required = append(required, t[1:])
		} else {
			optional = append(optional, t)
		}
	}

	scoringTerms := rawTerms
	if len(required) > 0 {
		scoringTerms = append(required, optional...)
	}

	patterns := make(map[string]*regexp.Regexp, len(scoringTerms))
	for _, term := range scoringTerms {
		if _, ok := patterns[term]; !ok {
			patterns[term] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
		}
	}

	type scored struct {
		tool  toolTypes.Tool
		score int
	}

	var candidates []scored

	for _, tool := range tools {
		nameLower := strings.ToLower(tool.Function.Name)
		descLower := strings.ToLower(tool.Function.Description)

		if nameLower == queryLower {
			candidates = append(candidates, scored{tool, 9999})
			continue
		}

		parts := strings.Split(nameLower, "_")

		if len(required) > 0 {
			ok := true
			for _, req := range required {
				pat := patterns[req]
				partMatch := false
				for _, p := range parts {
					if p == req || strings.Contains(p, req) {
						partMatch = true
						break
					}
				}
				if !partMatch && !pat.MatchString(descLower) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}

		score := 0
		for _, term := range scoringTerms {
			pat := patterns[term]

			exactPart := false
			for _, p := range parts {
				if p == term {
					exactPart = true
					break
				}
			}
			if exactPart {
				score += 10
				continue
			}

			partialPart := false
			for _, p := range parts {
				if strings.Contains(p, term) {
					partialPart = true
					break
				}
			}
			if partialPart {
				score += 5
				continue
			}

			if strings.Contains(nameLower, term) {
				score += 3
				continue
			}

			if pat.MatchString(descLower) {
				score += 2
			}
		}

		if score > 0 {
			candidates = append(candidates, scored{tool, score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if maxResults > 0 && len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	out := make([]result, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, result{
			Name:        c.tool.Function.Name,
			Description: c.tool.Function.Description,
		})
	}
	return out
}

func selectByName(names string, tools []toolTypes.Tool) []result {
	index := make(map[string]toolTypes.Tool, len(tools))
	for _, t := range tools {
		index[strings.ToLower(t.Function.Name)] = t
	}

	var out []result
	seen := make(map[string]bool)
	for _, raw := range strings.Split(names, ",") {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if t, ok := index[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, result{
				Name:        t.Function.Name,
				Description: t.Function.Description,
			})
		}
	}
	return out
}
