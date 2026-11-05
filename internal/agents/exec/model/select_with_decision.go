package model

import (
	"cmp"
	"context"
	"slices"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/session/history"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	decisionNamedNone    = "none"
	decisionTopicSame    = "same"
	DecisionLastModelKey = "lastModel:"
)

var decisionWorkCriteria = map[string]any{
	"code": map[string]any{
		"what":    decisionWorkWhat("code"),
		"not_for": "Explaining, reviewing or planning code without changing it.",
		"examples": []string{
			"這段有 race condition 嗎？幫我修掉",
			"Add a retry to the upload handler and write a test for it",
			"幫我把這個函式改成非同步並補上錯誤處理",
		},
	},
	"chat": map[string]any{
		"what":    decisionWorkWhat("chat"),
		"not_for": "Anything that needs tools, research or more than a few sentences.",
		"examples": []string{
			"早安",
			"Translate this paragraph into Japanese",
			"謝謝",
		},
	},
	"fetch": map[string]any{
		"what":    decisionWorkWhat("fetch"),
		"not_for": "Comparing, explaining or drawing conclusions from the data.",
		"examples": []string{
			"查一下現在的天氣",
			"What is this ticker trading at right now?",
			"列出這個資料夾的檔案",
		},
	},
	"research": map[string]any{
		"what":    decisionWorkWhat("research"),
		"not_for": "Fetching one value and returning it as-is, or a task that needs no investigation.",
		"examples": []string{
			"比較這三家雲端供應商的價格與限制",
			"Research how this regulation affects the products we ship",
			"分析這家公司近四季財報的趨勢",
		},
	},
	"work": map[string]any{
		"what":    decisionWorkWhat("work"),
		"not_for": "A request that merely sounds important or long but is really one of the other options.",
		"examples": []string{
			"Review this design doc and list the risks",
			"幫我規劃下週的發版流程",
			"把這段會議紀錄整理成待辦清單",
		},
	},
}

func selectWithDecision(ctx context.Context, meta SelectMeta, tagMap map[string]string) ([]string, string, int, error) {
	// * step 1: get decision agent
	agent, agentName, err := decision.Agent(ctx)
	if err != nil {
		return nil, "", 0, err
	}

	if len(meta.ModelNames) == 0 {
		return meta.ModelNames, "", 0, nil
	}

	named := map[string]any{decisionNamedNone: "The user does not mention any model."}
	for _, name := range meta.ModelNames {
		named[name] = "The request asks to use " + name + " (fuzzy match on provider, family or version)."
	}

	turns := decisionContext(meta.SessionID)
	questions := map[string]go_llm_router.DecisionQuestion{
		"work": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "What kind of work does `request` ask for?",
				"focus":    "Classify `request`; use `context` only to resolve references such as 'this' or 'continue'. When `skill` is present, `request` runs that Skill: classify the work `skill` describes, not its name.",
			},
			Criteria: decisionWorkCriteria,
		},
		"named": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Does `request` explicitly ask to use a specific model, such as \"use/with <name>\" or 指定/用 <名稱>?",
				"focus":    "Only an explicit instruction about which model to use counts; a model merely mentioned as a topic is none.",
			},
			Criteria: named,
		},
	}

	var previous, previousReasoning string
	if len(turns) > 0 {
		previous, previousReasoning = decisionPrevious(ctx, meta.SessionID, meta.ModelNames)
	}
	if previous != "" {
		questions["topic"] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Does `request` continue the subject of the last user message in `context`?",
				"focus":    "Compare only with the last user message in `context` and its reply; earlier turns do not count. Follow-ups, corrections, refinements and next steps on that subject are same, even when the kind of work changes.",
			},
			Criteria: map[string]any{
				decisionTopicSame: "`request` follows up on, refers to or builds on the last user message in `context` and its reply.",
				"new":             "`request` changes to a different subject than the last user message in `context`, even if an earlier turn discussed it.",
			},
		}
	}

	state := map[string]any{
		"context": turns,
		"request": strings.TrimSpace(meta.Task),
	}
	if meta.SkillDescription != "" {
		state["skill"] = meta.SkillDescription
	}
	answers, err := decision.Decide(ctx, agent, state, questions)
	if err != nil {
		return nil, "", 0, utilsmsg.Trace("Decide", agentName, err)
	}

	work := answers["work"].Choice
	order, ok := config.WorkTiers(work)
	if !ok {
		return nil, "", 0, utilsmsg.New(utilsmsg.Upstream, "work", "invalid choice %q", work)
	}

	list := make([]string, 0, len(meta.ModelNames)+1)
	if choice := answers["named"].Choice; choice != decisionNamedNone && named[choice] != nil {
		list = append(list, choice)
	}
	level := config.WorkReasoning(work)
	if previous != "" && answers["topic"].Choice == decisionTopicSame && (len(list) == 0 || list[0] == previous) {
		if _, ok := go_llm_router.ParseReasoning(previousReasoning); ok {
			level = previousReasoning
		}
		if len(list) == 0 {
			list = append(list, previous)
		}
	}
	pinned := len(list)

	for _, name := range rankCandidates(work, order, tagMap, meta.ModelNames) {
		if !slices.Contains(list, name) {
			list = append(list, name)
		}
	}
	return list, level, pinned, nil
}

func decisionWorkWhat(key string) string {
	i := slices.IndexFunc(config.WorkKinds, func(k config.WorkKind) bool { return k.Key == key })
	if i < 0 {
		return ""
	}
	return config.WorkKinds[i].What
}

func DecisionLastModelTTL(name string) int64 {
	prov, _, _ := strings.Cut(name, "@")
	switch prov {
	case "openai", "codex":
		return 30 * 60
	case "gemini":
		return 60 * 60
	}
	return 5 * 60
}

func decisionPrevious(ctx context.Context, sessionID string, candidates []string) (model, reasoning string) {
	if sessionID == "" {
		return "", ""
	}
	entry, ok := torii.DB(torii.DBToolCache).Get(ctx, DecisionLastModelKey+sessionID)
	if !ok {
		return "", ""
	}
	value := entry.Value()
	i := strings.LastIndex(value, "/")
	if i < 0 || !slices.Contains(candidates, value[:i]) {
		return "", ""
	}
	return value[:i], value[i+1:]
}

func decisionContext(sessionID string) []map[string]string {
	var records []history.Record
	if sessionID != "" {
		_, records = history.Get(sessionID)
	}

	list := []map[string]string{}
	for _, r := range slices.Backward(records) {
		if len(list) >= configs.MAX_DECISION_HISTORY_MESSAGES {
			break
		}
		if r.Role != "user" && r.Role != "assistant" {
			continue
		}
		text := strings.TrimSpace(configs.MESSAGE_PREFIX_REGEX.ReplaceAllString(r.Text(), ""))
		if text == "" {
			continue
		}
		if runes := []rune(text); len(runes) > configs.MAX_DECISION_RUNES {
			text = string(runes[:configs.MAX_DECISION_RUNES]) + "..."
		}
		list = append(list, map[string]string{"role": r.Role, "content": text})
	}
	slices.Reverse(list)
	return list
}

func rankCandidates(work string, order []string, tiers map[string]string, candidates []string) []string {
	rank := func(name string) int {
		tier, family := config.NameTier(name)
		i := slices.Index(order, cmp.Or(tiers[name], tier))
		if i < 0 {
			i = len(order)
		}
		return i*1000 + config.ProviderPreference(name)*100 + family
	}
	ranked := slices.Clone(candidates)
	slices.SortStableFunc(ranked, func(a, b string) int {
		return cmp.Or(
			cmp.Compare(rank(a), rank(b)),
			cmp.Compare(config.LongContextOrder(work, a), config.LongContextOrder(work, b)),
		)
	})
	return ranked
}
