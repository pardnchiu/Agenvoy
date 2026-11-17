package config

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/pardnchiu/go-llm-router/core/xai"

	"github.com/pardnchiu/agenvoy/configs"
)

type WorkKind struct {
	Key         string
	What        string
	Tiers       []string
	Reasoning   string
	LongContext bool
}

var WorkKinds = []WorkKind{
	{
		Key:       "code",
		Reasoning: "xhigh",
		What:      "Writing, fixing, debugging or testing code; a request that asks outright for depth or precision; a Skill that builds or tests code, or creates, installs, migrates or probes something.",
		Tiers:     []string{"S", "A", "B", "C"},
	},
	{
		Key:         "research",
		Reasoning:   "high",
		What:        "Research, analysis, comparison and reports: gathering from several sources or data points, then synthesizing findings and drawing conclusions.",
		Tiers:       []string{"S", "A", "B", "C"},
		LongContext: true,
	},
	{
		Key:       "work",
		Reasoning: "medium",
		What:      "General tasks: planning, reviewing, drafting, editing or organizing content the user already has, and anything that fits none of the other options.",
		Tiers:     []string{"A", "S", "B", "C"},
	},
	{
		Key:       "chat",
		Reasoning: "none",
		What:      "Greeting, small talk, a short factual answer, or translation.",
		Tiers:     []string{"B", "C", "A", "S"},
	},
	{
		Key:       "fetch",
		Reasoning: "low",
		What:      "Calling tools to fetch data and returning it without judgement; a Skill with fixed input and a deterministic transform.",
		Tiers:     []string{"C", "B", "A", "S"},
	},
}

func WorkReasoning(key string) string {
	i := slices.IndexFunc(WorkKinds, func(k WorkKind) bool { return k.Key == key })
	if i < 0 {
		return ""
	}
	return WorkKinds[i].Reasoning
}

func WorkTiers(key string) ([]string, bool) {
	i := slices.IndexFunc(WorkKinds, func(k WorkKind) bool { return k.Key == key })
	if i < 0 {
		return nil, false
	}
	return WorkKinds[i].Tiers, true
}

func NameTier(name string) (string, int) {
	model := strings.ToLower(name[strings.Index(name, "@")+1:])
	model = model[strings.LastIndex(model, "/")+1:]

	hasPrefix := func(prefixes ...string) bool {
		return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(model, p) })
	}
	family := func(prefix, tag string) bool {
		return strings.HasPrefix(model, prefix) && strings.Contains(model, tag)
	}

	switch {
	case strings.Contains(model, "-mini"), strings.Contains(model, "-nano"), family("gemini-", "-flash-lite"),
		hasPrefix("gemma", "gpt-oss", "qwen", "llama"):
		return "C", 30
	case hasPrefix("claude-fable"):
		return "S", 0
	case hasPrefix("claude-opus"):
		return "S", 1
	case family("gpt-", "-astra"):
		return "S", 2
	case family("gemini-", "-carbon"):
		return "S", 3
	case family("gpt-", "-sol"):
		return "A", 10
	case hasPrefix("grok-"):
		var version float64
		fmt.Sscanf(strings.TrimPrefix(model, "grok-"), "%f", &version)
		if version >= 4.5 {
			return "A", 11
		}
		return "B", 24
	case hasPrefix("claude-sonnet"):
		return "A", 12
	case family("gpt-", "-terra"):
		return "A", 13
	case family("gemini-", "-argon"):
		return "A", 14
	case family("gemini-", "-pro"):
		return "A", 15
	case family("deepseek-", "-pro"):
		return "A", 16
	case hasPrefix("glm"):
		return "A", 17
	case hasPrefix("kimi"):
		return "A", 18
	case hasPrefix("claude-haiku"):
		return "B", 20
	case family("gpt-", "-luna"):
		return "B", 21
	case family("gemini-", "-hydrogen"):
		return "B", 22
	case family("gemini-", "-flash"):
		return "B", 23
	case family("deepseek-", "-flash"):
		return "B", 25
	case hasPrefix("deepseek"):
		return "B", 26
	}
	return "A", 19
}

func ModelTier(tags map[string]string, name string) string {
	if tag := tags[name]; tag != "" {
		return tag
	}
	tier, _ := NameTier(name)
	return tier
}

const smallWindowProvider = "copilot"

var providerRank = map[string]int{
	"claude-code": 0,
	"codex":       1,
	"grok-oauth":  1,
	"copilot":     2,
	"openrouter":  4,
}

var preferredProviders = []string{"claude-code", "codex"}

func LongContextOrder(work, name string) int {
	i := slices.IndexFunc(WorkKinds, func(k WorkKind) bool { return k.Key == work })
	if i < 0 || !WorkKinds[i].LongContext {
		return 0
	}
	if prov, _, _ := strings.Cut(name, "@"); prov == smallWindowProvider {
		return 1
	}
	return 0
}

func ProviderOrder(name string) int {
	prov, _, _ := strings.Cut(name, "@")
	if rank, ok := providerRank[prov]; ok {
		return rank
	}
	return 3
}

func OrderByRank(models []string) []string {
	indexes := make(map[string][]int, len(models))
	for i, n := range models {
		key := baseModel(n)
		indexes[key] = append(indexes[key], i)
	}

	newModels := slices.Clone(models)
	for _, idx := range indexes {
		if len(idx) < 2 {
			continue
		}
		group := make([]string, len(idx))
		for i, at := range idx {
			group[i] = models[at]
		}
		slices.SortStableFunc(group, func(a, b string) int { return ProviderOrder(a) - ProviderOrder(b) })
		for i, at := range idx {
			newModels[at] = group[i]
		}
	}
	return newModels
}

func baseModel(name string) string {
	prov, model, ok := strings.Cut(name, "@")
	if !ok {
		return name
	}
	if prov == "openrouter" {
		if _, rest, found := strings.Cut(model, "/"); found {
			model = rest
		}
	}
	return strings.ToLower(model)
}

func ProviderPreference(name string) int {
	prov, _, _ := strings.Cut(name, "@")
	if i := slices.Index(preferredProviders, prov); i >= 0 {
		return i
	}
	return len(preferredProviders)
}

var lowerTier = map[string]string{"S": "A", "A": "B", "B": "C", "C": "C"}

func SubagentTiers(order []string) []string {
	list := make([]string, 0, len(order))
	for _, tier := range order {
		if lower := lowerTier[tier]; !slices.Contains(list, lower) {
			list = append(list, lower)
		}
	}
	for _, tier := range order {
		if !slices.Contains(list, tier) {
			list = append(list, tier)
		}
	}
	return list
}

func SelectionPrompt(cfg *Config, isSubagent bool) string {
	groups := make(map[string][]string, len(ModelTags))
	for _, m := range cfg.Models {
		tier := ModelTier(cfg.ModelTag, m.Name)
		groups[tier] = append(groups[tier], m.Name)
	}

	tierLines := make([]string, 0, len(ModelTags))
	for _, tier := range ModelTags {
		names := groups[tier]
		if len(names) == 0 {
			continue
		}
		slices.SortStableFunc(names, func(a, b string) int {
			_, fa := NameTier(a)
			_, fb := NameTier(b)
			return cmp.Or(cmp.Compare(ProviderPreference(a), ProviderPreference(b)), cmp.Compare(fa, fb), cmp.Compare(ProviderOrder(a), ProviderOrder(b)))
		})
		tierLines = append(tierLines, fmt.Sprintf("- %s: %s", tier, strings.Join(names, ", ")))
	}
	modelTier := "(no models registered)"
	if len(tierLines) > 0 {
		modelTier = strings.Join(tierLines, "\n")
	}

	workLines := make([]string, 0, len(WorkKinds))
	for _, k := range WorkKinds {
		order := k.Tiers
		if isSubagent {
			order = SubagentTiers(order)
		}
		workLines = append(workLines, fmt.Sprintf("- %s (%s): %s", k.Key, strings.Join(order, " > "), k.What))
	}

	xSearch := ""
	var xModels []string
	for _, m := range cfg.Models {
		if prov, model, _ := strings.Cut(m.Name, "@"); (prov == "grok-oauth" || prov == "grok") && xai.SupportsXSearch(model) && ModelTier(cfg.ModelTag, m.Name) != ModelTagPass {
			xModels = append(xModels, m.Name)
		}
	}
	if len(xModels) > 0 {
		xSearch = fmt.Sprintf("\n\nA request about X (Twitter), its posts, accounts, threads or what people say there, puts these models first whatever the kind of work, since only they search X directly: %s.", strings.Join(xModels, ", "))
	}

	return strings.NewReplacer(
		"{{.ModelTier}}", modelTier,
		"{{.WorkTiers}}", strings.Join(workLines, "\n"),
		"{{.XSearch}}", xSearch,
	).Replace(strings.TrimSpace(configs.ModelSelection))
}
