package hook

import (
	"encoding/json"
	"strings"
)

type Answer struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func Answers(args, result string) []Answer {
	var input struct {
		Questions []struct {
			Question string `json:"question"`
		} `json:"questions"`
	}
	var output struct {
		Answers []any `json:"answers"`
	}
	_ = json.Unmarshal([]byte(args), &input)
	_ = json.Unmarshal([]byte(result), &output)

	list := make([]Answer, 0, len(input.Questions))
	for i, q := range input.Questions {
		answer := ""
		if i < len(output.Answers) {
			answer = answerText(output.Answers[i])
		}
		list = append(list, Answer{Question: strings.TrimSpace(q.Question), Answer: answer})
	}
	return list
}

func answerText(answer any) string {
	switch v := answer.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}
