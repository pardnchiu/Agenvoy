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
	"golang.org/x/term"

	"github.com/pardnchiu/agenvoy/internal/pending"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type askQuestion struct {
	Question    string   `json:"question"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

func registAskUser() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "ask_user",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Description: "Ask the user one or more questions and return their answers.",
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
							"secret": map[string]any{
								"type":        "boolean",
								"description": "Free-text only: true masks input and excludes the answer from logs. Ignored when options is set.",
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

			if pending.Active.Load() {
				questions := make([]pending.Question, 0, len(params.Questions))
				for i, q := range params.Questions {
					q.Question = strings.TrimSpace(q.Question)
					if q.Question == "" {
						return "", fmt.Errorf("question #%d is empty", i+1)
					}
					questions = append(questions, pending.Question{
						Question:    q.Question,
						Options:     q.Options,
						MultiSelect: q.MultiSelect,
						Secret:      q.Secret,
					})
				}
				reply, err := pending.Ask(ctx, pending.Request{
					Kind:      pending.KindAskUser,
					SessionID: e.SessionID,
					ToolName:  "ask_user",
					AskUser:   &pending.UserPayload{Questions: questions},
				})
				if err != nil {
					return "", fmt.Errorf("pending.Ask: %w", err)
				}
				if reply.Error != nil {
					return "", reply.Error
				}
				out, err := json.Marshal(map[string]any{"answers": reply.Answers})
				if err != nil {
					return "", fmt.Errorf("json.Marshal: %w", err)
				}
				return string(out), nil
			}

			if !strings.HasPrefix(e.SessionID, "cli-") {
				var sb strings.Builder
				sb.WriteString("此 session 無互動 stdin（非 cli- session），無法即時收使用者輸入。請在你接下來的回覆中，直接以自然語言把以下問題傳達給使用者，等待使用者下一則訊息提供答案後再繼續：\n")
				for i, q := range params.Questions {
					question := strings.TrimSpace(q.Question)
					if question == "" {
						continue
					}
					fmt.Fprintf(&sb, "%d. %s", i+1, question)
					if len(q.Options) > 0 {
						sb.WriteString("（選項：")
						sb.WriteString(strings.Join(q.Options, " / "))
						if q.MultiSelect {
							sb.WriteString("，可複選）")
						} else {
							sb.WriteString("）")
						}
					}
					sb.WriteString("\n")
				}
				sb.WriteString("禁止自行猜測答案或代為填入預設值；缺資訊就先停下來問。")
				return sb.String(), nil
			}

			reader := bufio.NewReader(os.Stdin)
			answers := make([]any, 0, len(params.Questions))
			for i, q := range params.Questions {
				q.Question = strings.TrimSpace(q.Question)
				if q.Question == "" {
					return "", fmt.Errorf("question #%d is empty", i+1)
				}

				switch {
				case len(q.Options) == 0 && q.Secret:
					ans, err := askWithSecretInput(q.Question)
					if err != nil {
						return "", err
					}
					answers = append(answers, ans)

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

func askWithSecretInput(question string) (string, error) {
	if _, err := fmt.Fprintf(os.Stdout, "[?] %s: ", question); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	raw, readErr := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if readErr != nil {
		return "", fmt.Errorf("term.ReadPassword: %w", readErr)
	}
	return strings.TrimSpace(string(raw)), nil
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
