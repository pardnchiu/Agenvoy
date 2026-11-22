package file

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
)

const (
	blockUnrelated      = "unrelated"
	MIN_RELEVANCE_RUNES = 1 << 14
	WITHHELD_MARKER     = "read it again with full: true to load the whole file]"
)

var (
	blankLineRegex = regexp.MustCompile(`^(\d+\t)?\s*$`)
	rowPrefixRegex = regexp.MustCompile(`^(\d+)\t`)
	blockCriteria  = map[string]any{
		"related":      "The block helps answer `request`, even partly, or gives context needed to read a block that does.",
		blockUnrelated: "The block is about something `request` does not ask about.",
	}
)

func checkFileRelevance(ctx context.Context, sessionID, objective, path, content string) string {
	if utf8.RuneCountInString(content) < MIN_RELEVANCE_RUNES {
		if !decision.Unrelated(ctx, objective, content, slog.String("session", sessionID), slog.String("path", path)) {
			return content
		}
		return fileWithheld(path)
	}
	request, ok := decision.Request(objective)
	if !ok {
		return content
	}
	agent, name, ok := decision.Ready(ctx, slog.String("session", sessionID), slog.String("path", path))
	if !ok {
		return content
	}

	blocks := splitBlocks(content)
	dropped := make([]bool, len(blocks))
	slots := make(chan struct{}, configs.MAX_DECISION_PARALLEL)
	var wg sync.WaitGroup
	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
	for _, batch := range decision.Batch(len(blocks), budget, func(i int) int { return utf8.RuneCountInString(blocks[i]) }) {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			state := make(map[string]string, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				key := fmt.Sprintf("b%d", i)
				state[key] = blocks[i]
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": fmt.Sprintf("Does block `%s` in `blocks` help answer `request`?", key),
						"focus":    "Judge the block against `request`; when unsure, keep it as related.",
					},
					Criteria: blockCriteria,
				}
			}
			answers, err := decision.Decide(ctx, agent, map[string]any{"request": request, "blocks": state}, questions)
			if err != nil {
				slog.Debug("[Unavailable] read_files relevance filter: keep batch",
					slog.String("session", sessionID),
					slog.String("path", path),
					slog.String("model", name),
					slog.String("error", err.Error()))
				return
			}
			for _, i := range batch {
				dropped[i] = decision.Confident(answers[fmt.Sprintf("b%d", i)], blockUnrelated)
			}
		})
	}
	wg.Wait()

	var sb strings.Builder
	keptCount := 0
	for i := 0; i < len(blocks); {
		if !dropped[i] {
			sb.WriteString(blocks[i])
			keptCount++
			i++
			continue
		}
		j := i
		for j < len(blocks) && dropped[j] {
			j++
		}
		sb.WriteString(withheldMarker(blocks[i:j]))
		i = j
	}
	slog.Debug("read_files relevance filter",
		slog.String("session", sessionID),
		slog.String("path", path),
		slog.Int("blocks", len(blocks)),
		slog.Int("dropped", len(blocks)-keptCount))

	if keptCount == len(blocks) {
		return content
	}
	if keptCount == 0 {
		return fileWithheld(path)
	}
	return sb.String()
}

func fileWithheld(path string) string {
	return fmt.Sprintf("File withheld: %s does not bear on the current request, so its content was not loaded. If it is needed after all, read it again with full: true to load the whole file.", path)
}

func withheldMarker(blocks []string) string {
	text := strings.Join(blocks, "")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	first := rowPrefixRegex.FindStringSubmatch(lines[0])
	last := rowPrefixRegex.FindStringSubmatch(lines[len(lines)-1])
	if first != nil && last != nil {
		return fmt.Sprintf("[unrelated: rows %s-%s not loaded; %s\n", first[1], last[1], WITHHELD_MARKER)
	}
	return fmt.Sprintf("[unrelated: %d lines not loaded; %s\n", len(lines), WITHHELD_MARKER)
}

func splitBlocks(content string) []string {
	var blocks []string
	var sb strings.Builder
	for line := range strings.SplitAfterSeq(content, "\n") {
		sb.WriteString(line)
		if blankLineRegex.MatchString(strings.TrimRight(line, "\r\n")) {
			blocks = append(blocks, sb.String())
			sb.Reset()
		}
	}
	if sb.Len() > 0 {
		blocks = append(blocks, sb.String())
	}
	return blocks
}
