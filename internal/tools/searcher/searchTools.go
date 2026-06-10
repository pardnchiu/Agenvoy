package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type ToolMatch struct {
	Injected   []Tool `json:"injected"`
	Query      string `json:"query"`
	TotalTools int    `json:"total_tools"`
}

func registSearchTools() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_tools",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Description: `
Search tool registry by keyword (or 'select:<name>' for exact activation) and inject schemas.
Use when a capability isn't loaded.
Prefer unmarked tools (mcp__* > script_* > api_*) over [system-default] for same intent.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": `Keywords (all must match), or "select:<name>,<name>" for exact activation.`,
				},
			},
			"required": []string{"query"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			if len(args) < 1 {
				return "", fmt.Errorf("arguments are required")
			}

			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}

			var matches []Tool
			if name, ok := strings.CutPrefix(params.Query, "select:"); ok {
				matches = matchName(name, e.AllTools)
			} else {
				matches = matchKeyword(params.Query, e.AllTools)
			}

			toolDic := make(map[string]toolTypes.Tool, len(e.AllTools))
			for _, tool := range e.AllTools {
				toolDic[tool.Function.Name] = tool
			}

			for _, match := range matches {
				if e.ExcludeTools[match.Name] {
					continue
				}

				full, ok := toolDic[match.Name]
				if !ok {
					continue
				}

				replaced := false
				for i, t := range e.Tools {
					if t.Function.Name == match.Name {
						e.Tools[i] = full
						replaced = true
						break
					}
				}
				if !replaced {
					e.Tools = append(e.Tools, full)
				}
				delete(e.StubTools, match.Name)
			}

			raw, err := json.Marshal(ToolMatch{
				Injected:   matches,
				Query:      params.Query,
				TotalTools: len(e.AllTools),
			})
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

func matchName(names string, tools []toolTypes.Tool) []Tool {
	dic := make(map[string]toolTypes.Tool, len(tools))
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
	case strings.HasPrefix(name, "api_"):
		return "api"
	case strings.HasPrefix(name, "script_"):
		return "script"
	case strings.HasPrefix(name, "ext_"):
		return "extension"
	default:
		return "sys"
	}
}

func matchKeyword(query string, tools []toolTypes.Tool) []Tool {
	query = strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil
	}

	dic := make(map[string]*regexp.Regexp, len(terms))
	for _, term := range terms {
		if _, ok := dic[term]; !ok {
			dic[term] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
		}
	}

	type scored struct {
		tool  toolTypes.Tool
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
			} else if pat.MatchString(desc) {
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

	sort.Slice(candidates, func(i, j int) bool {
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
			SystemDefault: strings.HasPrefix(strings.TrimSpace(candidate.tool.Function.Description), systemDefaultMarker),
		})
	}
	return list
}
