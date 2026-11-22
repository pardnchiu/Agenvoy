package fetchPage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	go_browser "github.com/pardnchiu/go-browser/core"
	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agentsdecision "github.com/pardnchiu/agenvoy/internal/agents/decision"
)

const (
	decisionNoise     = "noise"
	decisionUnrelated = "unrelated"
	decisionAnswer    = "answers"
	filteredMarker    = "[Filtered for the current request:"
)

var (
	blockSplitRegex = regexp.MustCompile(`\n[ \t]*\n`)
	titleRegex      = regexp.MustCompile(`(?m)^title: "(.*)"$`)
	noiseCriteria   = map[string]any{
		"keep":        "Page content a reader could need: article text, data, tables, figures, lists of facts or items that belong to the page's subject.",
		decisionNoise: "Page chrome that carries no information: ads, sponsored blocks, navigation or breadcrumb menus, share or subscribe prompts, cookie notices, 'read more' links, footers, related-article link lists.",
	}
	relevantCriteria = map[string]any{
		"related":         "The block helps answer `request`, even partly, or gives context needed to read a block that does.",
		decisionUnrelated: "The block is about something `request` does not ask about.",
	}
	answerCriteria = map[string]any{
		decisionAnswer:    "The block itself states the answer `request` asks for: the definition, value, rule or explanation, not just a mention of the subject.",
		decisionUnrelated: "The block does not state that answer: it only mentions the subject, gives general background, or covers something else.",
	}
)

type jsonPage struct {
	Title string            `json:"title"`
	Tree  []json.RawMessage `json:"tree"`
}

type blockFilter struct {
	label    string
	question string
	focus    string
	criteria map[string]any
	drop     string
	state    map[string]any
	budget   int
	skip     func(block string) bool
	keep     string
	minKeep  float64
}

func filterBlocks(ctx context.Context, link string, blocks []string, f blockFilter) ([]string, int) {
	agent, name, ok := agentsdecision.Ready(ctx, slog.String("url", link))
	if !ok {
		return blocks, -1
	}

	dropped := make([]bool, len(blocks))
	slots := make(chan struct{}, configs.MAX_DECISION_PARALLEL)
	var wg sync.WaitGroup
	for _, batch := range agentsdecision.Batch(len(blocks), f.budget, func(i int) int { return utf8.RuneCountInString(blocks[i]) }) {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			state := make(map[string]string, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				if f.skip != nil && f.skip(blocks[i]) {
					continue
				}
				key := fmt.Sprintf("b%d", i)
				state[key] = blocks[i]
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": strings.ReplaceAll(f.question, "{key}", key),
						"focus":    f.focus,
					},
					Criteria: f.criteria,
				}
			}
			if len(questions) == 0 {
				return
			}
			input := map[string]any{"blocks": state}
			for k, v := range f.state {
				input[k] = v
			}
			answers, err := agentsdecision.Decide(ctx, agent, input, questions)
			if err != nil {
				slog.Debug("[Unavailable] fetch_page "+f.label+": keep batch",
					slog.String("url", link),
					slog.String("model", name),
					slog.String("error", err.Error()))
				return
			}
			for _, i := range batch {
				if f.skip != nil && f.skip(blocks[i]) {
					continue
				}
				answer := answers[fmt.Sprintf("b%d", i)]
				if f.minKeep > 0 {
					dropped[i] = answer.Choice != f.keep || answer.Confidence == nil || *answer.Confidence < f.minKeep
					continue
				}
				dropped[i] = agentsdecision.Confident(answer, f.drop)
			}
		})
	}
	wg.Wait()

	kept := make([]string, 0, len(blocks))
	judged := 0
	for i, block := range blocks {
		if dropped[i] {
			continue
		}
		kept = append(kept, block)
		if f.skip == nil || !f.skip(block) {
			judged++
		}
	}
	slog.Debug("fetch_page "+f.label,
		slog.String("url", link),
		slog.Int("blocks", len(blocks)),
		slog.Int("dropped", len(blocks)-len(kept)))
	return kept, judged
}

func removeNoise(ctx context.Context, link, content string) string {
	blocks := blockSplitRegex.Split(strings.TrimSpace(content), -1)
	if len(blocks) < 2 {
		return content
	}
	kept, _ := filterBlocks(ctx, link, blocks, blockFilter{
		label:    "noise filter",
		question: "Is block `{key}` in `blocks` noise?",
		focus:    "Judge the block by itself; when unsure, keep.",
		criteria: noiseCriteria,
		drop:     decisionNoise,
		budget:   configs.MAX_DECISION_STATE_RUNES,
		skip:     isTable,
	})
	return strings.Join(kept, "\n\n")
}

func isTable(block string) bool {
	return strings.HasPrefix(strings.TrimSpace(block), "|")
}

func relevantBlocks(ctx context.Context, objective, link string, blocks []string, skip func(string) bool, huge bool) ([]string, int) {
	request, ok := agentsdecision.Request(objective)
	if !ok || len(blocks) < 2 {
		return blocks, -1
	}
	f := blockFilter{
		label:    "relevance filter",
		question: "Does block `{key}` in `blocks` help answer `request`?",
		focus:    "Judge the block against `request`; when unsure, keep it as related.",
		criteria: relevantCriteria,
		drop:     decisionUnrelated,
		state:    map[string]any{"request": request},
		budget:   configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request),
		skip:     skip,
	}
	if huge {
		f.label = "strict relevance filter"
		f.question = "Does block `{key}` in `blocks` itself state the answer to `request`?"
		f.focus = "Mark it answers only when the block contains the answer; a mention of the subject or related background is unrelated."
		f.criteria = answerCriteria
		f.keep = decisionAnswer
		f.minKeep = configs.DECISION_MIN_CONFIDENCE
		f.budget = min(f.budget, configs.MAX_DECISION_STATE_RUNES/4)
	}
	return filterBlocks(ctx, link, blocks, f)
}

func keepRelevant(ctx context.Context, objective, link, content string, huge, keepLinks bool) (string, bool) {
	blocks := splitBlocks(content)
	kept, related := relevantBlocks(ctx, objective, link, blocks, func(block string) bool {
		return isTable(block) ||
			(!huge && strings.HasPrefix(strings.TrimSpace(block), "#")) ||
			(keepLinks && strings.Contains(block, "]("))
	}, huge)
	if related == 0 {
		return "", true
	}
	scope := ""
	if huge {
		scope = " from the first 1 MiB of a larger page"
	}
	if related < 0 && huge {
		return fmt.Sprintf("[Only the first 1 MiB of a larger page; %s]\n\n%s", fullPageHint, content), false
	}
	if related < 0 || (len(kept) == len(blocks) && !huge) {
		return content, false
	}
	return fmt.Sprintf("%s kept %d of %d blocks%s; %s]\n\n%s", filteredMarker, len(kept), len(blocks), scope, fullPageHint, strings.Join(kept, "\n\n")), false
}

func flattenNodes(tree []json.RawMessage) []json.RawMessage {
	list := make([]json.RawMessage, 0, len(tree))
	for _, raw := range tree {
		var node struct {
			Type     string            `json:"type"`
			Children []json.RawMessage `json:"children"`
		}
		if err := json.Unmarshal(raw, &node); err == nil && len(node.Children) > 0 &&
			(node.Type == "container" || utf8.RuneCount(raw) > configs.MAX_DECISION_STATE_RUNES/2) {
			list = append(list, flattenNodes(node.Children)...)
			continue
		}
		list = append(list, raw)
	}
	return list
}

func splitBlocks(content string) []string {
	list := []string{}
	for _, block := range blockSplitRegex.Split(strings.TrimSpace(content), -1) {
		if isTable(block) || utf8.RuneCountInString(block) <= configs.MAX_DECISION_STATE_RUNES/2 {
			list = append(list, block)
			continue
		}
		list = append(list, strings.Split(block, "\n")...)
	}
	return list
}

func keepRelevantNodes(ctx context.Context, objective, link string, tree []json.RawMessage, huge, keepLinks bool) ([]json.RawMessage, bool) {
	nodes := flattenNodes(tree)
	if huge {
		size := 0
		for i, node := range nodes {
			if size += len(node); size > maxFetchLength {
				nodes = nodes[:i]
				break
			}
		}
	}
	blocks := make([]string, len(nodes))
	for i, node := range nodes {
		blocks[i] = string(node)
	}
	kept, related := relevantBlocks(ctx, objective, link, blocks, func(block string) bool {
		return (!huge && strings.HasPrefix(block, `{"type":"heading"`)) ||
			(keepLinks && strings.Contains(block, `"href"`))
	}, huge)
	if related == 0 {
		return nil, true
	}
	if related < 0 && huge {
		return nodes, false
	}
	if related < 0 || (len(kept) == len(blocks) && !huge) {
		return tree, false
	}
	list := make([]json.RawMessage, len(kept))
	for i, block := range kept {
		list[i] = json.RawMessage(block)
	}
	return list, false
}

func deliverPage(ctx context.Context, objective, link string, outType int, keepLinks bool, result string) string {
	switch outType {
	case go_browser.TypeMarkdown:
		header, body := splitFrontmatter(result)
		huge := len(body) > maxFetchLength
		if huge {
			body = headBytes(body, maxFetchLength)
		}
		page := header + body
		if !huge && utf8.RuneCountInString(page) <= wholePageLimit(objective) {
			return checkRelevance(ctx, objective, link, page)
		}
		content, unrelated := keepRelevant(ctx, objective, link, body, huge, keepLinks)
		if unrelated {
			title := ""
			if m := titleRegex.FindStringSubmatch(header); m != nil {
				title = m[1]
			}
			return withheld(link, title, huge)
		}
		return header + content
	case go_browser.TypeJSON:
		var page jsonPage
		if err := json.Unmarshal([]byte(result), &page); err != nil || page.Tree == nil {
			if err := json.Unmarshal([]byte(result), &page.Tree); err != nil {
				return result
			}
		}
		tree, err := json.Marshal(page.Tree)
		if err != nil {
			return result
		}
		huge := len(tree) > maxFetchLength
		if !huge && utf8.RuneCount(tree) <= wholePageLimit(objective) {
			if agentsdecision.Unrelated(ctx, objective, string(tree), slog.String("url", link)) {
				return withheld(link, page.Title, false)
			}
			return string(tree)
		}
		nodes, unrelated := keepRelevantNodes(ctx, objective, link, page.Tree, huge, keepLinks)
		if unrelated {
			return withheld(link, page.Title, huge)
		}
		raw, err := json.Marshal(nodes)
		if err != nil {
			return string(tree)
		}
		return string(raw)
	}
	return result
}

func splitFrontmatter(page string) (string, string) {
	if !strings.HasPrefix(page, "---\n") {
		return "", page
	}
	i := strings.Index(page[4:], "\n---\n")
	if i < 0 {
		return "", page
	}
	end := 4 + i + 5
	return page[:end], page[end:]
}

func wholePageLimit(objective string) int {
	request, _ := agentsdecision.Request(objective)
	return configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
}

func checkRelevance(ctx context.Context, objective, link, result string) string {
	if utf8.RuneCountInString(result) > wholePageLimit(objective) {
		return result
	}
	if !agentsdecision.Unrelated(ctx, objective, result, slog.String("url", link)) {
		return result
	}

	title := ""
	if m := titleRegex.FindStringSubmatch(result); m != nil {
		title = m[1]
	}
	return withheld(link, title, false)
}

func withheld(link, title string, huge bool) string {
	if huge {
		return fmt.Sprintf("Web page withheld: %s (title: %q) is over 1 MiB and its first 1 MiB has nothing on the current request; %s, or use a different source.", link, title, fullPageHint)
	}
	return fmt.Sprintf("Web page withheld: %s (title: %q) does not bear on the current request, so its content was not loaded. Use a different source.", link, title)
}
