package toolSearcher

import (
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type ToolMatch struct {
	Injected   []Tool `json:"injected"`
	Query      string `json:"query"`
	TotalTools int    `json:"total_tools"`
}

func searchTools(e *tooltypes.Executor, query string) (string, error) {
	var matches []Tool
	if name, ok := strings.CutPrefix(query, "select:"); ok {
		matches = matchName(name, e.AllTools)
	} else {
		matches = matchKeyword(query, e.AllTools)
	}

	e.ToolsMu.Lock()
	delivered := matches[:0]
	for _, match := range matches {
		if e.ExcludeTools[match.Name] {
			continue
		}
		delete(e.UnmarkedTools, match.Name)
		delivered = append(delivered, match)
	}
	matches = delivered
	e.ToolsMu.Unlock()

	raw, err := json.Marshal(ToolMatch{
		Injected:   matches,
		Query:      query,
		TotalTools: len(e.AllTools),
	})
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}

func matchName(names string, tools []go_llm_router.Tool) []Tool {
	dic := make(map[string]go_llm_router.Tool, len(tools))
	for _, tool := range tools {
		dic[strings.ToLower(tool.Function.Name)] = tool
	}

	var list []Tool
	dicSeen := make(map[string]bool)
	for name := range strings.SplitSeq(names, ",") {
		name := strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if tool, ok := dic[name]; ok && !dicSeen[name] {
			dicSeen[name] = true
			list = append(list, Tool{
				Name:          tool.Function.Name,
				Description:   tool.Function.Description,
				Parameters:    tool.Function.Parameters,
				SystemDefault: strings.HasPrefix(strings.TrimSpace(tool.Function.Description), systemDefaultMarker),
			})
		}
	}
	return list
}

func toolCategory(name string) string {
	switch {
	case strings.HasPrefix(name, "mcp__"):
		return "mcp"
	case strings.HasPrefix(name, "script_"):
		return "script"
	case strings.HasPrefix(name, "ext_"):
		return "extension"
	default:
		return "sys"
	}
}

func matchKeyword(query string, tools []go_llm_router.Tool) []Tool {
	query = strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil
	}

	dic := make(map[string]*regexp.Regexp, len(terms))
	for _, term := range terms {
		if _, ok := dic[term]; ok || !isASCII(term) {
			continue
		}
		dic[term] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
	}

	type scored struct {
		tool  go_llm_router.Tool
		score int
	}

	var candidates []scored
	for _, tool := range tools {
		name := strings.ToLower(tool.Function.Name)
		desc := strings.ToLower(tool.Function.Description)

		if name == query {
			candidates = append(candidates, scored{tool, 9999})
			continue
		}

		parts := strings.Split(name, "_")

		score := 0
		allHit := true
		for _, term := range terms {
			pat := dic[term]
			hit := false

			if slices.Contains(parts, term) {
				score += 10
				hit = true
			}
			if hit {
				continue
			}

			for _, p := range parts {
				if strings.Contains(p, term) {
					score += 5
					hit = true
					break
				}
			}
			if hit {
				continue
			}

			if strings.Contains(name, term) {
				score += 3
				hit = true
			} else if descHit(desc, term, pat) {
				score += 4
				hit = true
			}

			if !hit {
				allHit = false
				break
			}
		}

		if !allHit {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(tool.Function.Description), systemDefaultMarker) {
			score--
		}
		candidates = append(candidates, scored{tool, score})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	dicCount := map[string]int{}
	var list []Tool
	for _, candidate := range candidates {
		name := toolCategory(candidate.tool.Function.Name)
		if dicCount[name] >= 5 {
			continue
		}
		dicCount[name]++
		list = append(list, Tool{
			Name:          candidate.tool.Function.Name,
			Description:   candidate.tool.Function.Description,
			Parameters:    candidate.tool.Function.Parameters,
			SystemDefault: strings.HasPrefix(strings.TrimSpace(candidate.tool.Function.Description), systemDefaultMarker),
		})
	}
	return list
}

func isASCII(str string) bool {
	for i := range len(str) {
		if str[i] >= 0x80 {
			return false
		}
	}
	return true
}

func descHit(desc, term string, pat *regexp.Regexp) bool {
	if pat == nil {
		return strings.Contains(desc, term)
	}
	return pat.MatchString(desc)
}
