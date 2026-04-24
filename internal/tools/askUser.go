package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type askQuestion struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
}

func registAskUser() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "ask_user",
		ReadOnly:   true,
		AlwaysLoad: true,
		Description: `
Ask the user one or more questions and wait for typed/selected answers.
Use when a skill or flow needs runtime input from the user (e.g. config fields, confirmations, choices).
Only works in single-shot CLI mode (make cli/run); errors out in TUI/Discord/REST.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type":        "array",
					"description": "Questions to ask in order. Each item is either a free-text prompt (no options) or a choice prompt (with options).",
					"minItems":    1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question": map[string]any{
								"type":        "string",
								"description": "The prompt text shown to the user.",
							},
							"options": map[string]any{
								"type":        "array",
								"description": "Fixed choices. Omit or empty for free-text input.",
								"items":       map[string]any{"type": "string"},
							},
							"multi_select": map[string]any{
								"type":        "boolean",
								"description": "When options is non-empty: true = multi-select (comma-separated indices), false = single-select (arrow keys). Ignored for free-text.",
							},
						},
						"required": []string{"question"},
					},
				},
			},
			"required": []string{"questions"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Questions []askQuestion `json:"questions"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if len(params.Questions) == 0 {
				return "", fmt.Errorf("ask_user requires at least one question in 'questions'")
			}

			reader := bufio.NewReader(os.Stdin)
			answers := make([]any, 0, len(params.Questions))
			for i, q := range params.Questions {
				q.Question = strings.TrimSpace(q.Question)
				if q.Question == "" {
					return "", fmt.Errorf("question #%d is empty", i+1)
				}

				switch {
				case len(q.Options) == 0:
					ans, err := askWithInput(reader, q.Question)
					if err != nil {
						return "", err
					}
					answers = append(answers, ans)

				case q.MultiSelect:
					ans, err := askWithMultiSelect(reader, q.Question, q.Options, i+1)
					if err != nil {
						return "", err
					}
					answers = append(answers, ans)

				default:
					ans, err := askWithSingleSelect(q.Question, q.Options)
					if err != nil {
						return "", err
					}
					answers = append(answers, ans)
				}
			}

			out, err := json.Marshal(map[string]any{"answers": answers})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

func askWithInput(reader *bufio.Reader, question string) (string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s\n> ", question); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func askWithSingleSelect(question string, options []string) (string, error) {
	prompt := promptui.Select{
		Label:        question,
		Items:        options,
		Size:         len(options),
		HideSelected: false,
	}
	_, chosen, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("promptui.Select: %w", err)
	}
	return chosen, nil
}

func askWithMultiSelect(reader *bufio.Reader, question string, options []string, qIdx int) ([]string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s (multi-select, comma-separated indices)\n", question); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	for i, opt := range options {
		if _, err := fmt.Fprintf(os.Stdout, "  %d) %s\n", i+1, opt); err != nil {
			return nil, fmt.Errorf("write prompt: %w", err)
		}
	}
	if _, err := fmt.Fprint(os.Stdout, "> "); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	line = strings.TrimSpace(line)

	seen := make(map[int]bool, len(options))
	selected := make([]string, 0, len(options))
	for _, tok := range strings.Split(line, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		idx, err := strconv.Atoi(tok)
		if err != nil || idx < 1 || idx > len(options) {
			return nil, fmt.Errorf("invalid multi-select input %q for question #%d: expected comma-separated integers in 1..%d", line, qIdx, len(options))
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		selected = append(selected, options[idx-1])
	}
	return selected, nil
}
