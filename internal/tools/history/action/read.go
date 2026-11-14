package actionHistory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const withheldObjectiveRunes = 200

func read(ctx context.Context, e *tooltypes.Executor, taskIDs []string, full bool) (string, error) {
	list, err := entries(e)
	if err != nil {
		return "", err
	}

	idEntry := make(map[string]entry, len(list))
	for _, item := range list {
		idEntry[item.taskID] = item
	}

	blocks := make([]string, 0, len(taskIDs))
	ids := make([]string, 0, len(taskIDs))
	objectives := make([]string, 0, len(taskIDs))
	var missing []string
	for _, id := range taskIDs {
		item, ok := idEntry[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		r, err := load(item)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, render(item, r, full))
		ids = append(ids, id)
		objectives = append(objectives, r.Objective)
	}

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Go(func() {
			if decision.Withhold(ctx, e.SessionID, "task:"+id, e.Objective, blocks[i], slog.String("task", id)) {
				blocks[i] = fmt.Sprintf("task %s — withheld: this run does not bear on the current request, so its content was not loaded (objective: %q). Read it again only if it is needed after all; a second read returns the content.",
					id, go_pkg_utils.TruncateString(strings.Join(strings.Fields(objectives[i]), " "), withheldObjectiveRunes))
			}
		})
	}
	wg.Wait()

	if len(blocks) == 0 {
		return "", utilsmsg.New(utilsmsg.NotFound, "task", "no finished task with id %s in this session", strings.Join(missing, ", "))
	}
	out := strings.Join(blocks, "\n\n---\n\n")
	if len(missing) > 0 {
		out += fmt.Sprintf("\n\n---\n\nnot found in this session: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func render(item entry, r record, full bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "task %s — %s", item.taskID, item.at.Format(displayLayout))
	if r.Model != "" {
		fmt.Fprintf(&b, " · %s", r.Model)
	}
	if r.Reasoning != "" {
		fmt.Fprintf(&b, " · reasoning %s", r.Reasoning)
	}
	if r.Objective != "" {
		fmt.Fprintf(&b, "\n\nobjective:\n%s", r.Objective)
	}

	var withheld []string
	for _, t := range r.ToolResults {
		if !full && !isReference(t.Name) {
			withheld = append(withheld, t.Name)
			continue
		}
		if t.Args == "" {
			fmt.Fprintf(&b, "\n\ncalled %s:\n%s", t.Name, t.Result)
			continue
		}
		fmt.Fprintf(&b, "\n\ncalled %s(%s):\n%s", t.Name, t.Args, t.Result)
	}
	if len(withheld) > 0 {
		fmt.Fprintf(&b, "\n\nalso called, output withheld by scope=reference — scope=full to include it: %s",
			strings.Join(withheld, ", "))
	}

	if r.Reply != "" {
		fmt.Fprintf(&b, "\n\nreplied:\n%s", r.Reply)
	}
	return b.String()
}
