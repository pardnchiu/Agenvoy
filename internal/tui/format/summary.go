package format

import (
	"encoding/json"
	"fmt"
	"strings"
)

func Summary(data []byte, width int) string {
	var s struct {
		ConfirmedNeeds    []string `json:"confirmed_needs"`
		Constraints       []string `json:"constraints"`
		CoreDiscussion    string   `json:"core_discussion"`
		CurrentConclusion []string `json:"current_conclusion"`
		DiscussionLog     []struct {
			Time       string `json:"time"`
			Topic      string `json:"topic"`
			Conclusion string `json:"conclusion"`
		} `json:"discussion_log"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	divider := strings.Repeat("─", width/2)
	var sb strings.Builder
	writeSection := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		sb.WriteString(title + "\n")
		sb.WriteString(divider + "\n")
		for _, item := range items {
			sb.WriteString("  - " + item + "\n")
		}
		sb.WriteString("\n")
	}
	writeSection("CONFIRMED NEEDS", s.ConfirmedNeeds)
	writeSection("CONSTRAINTS", s.Constraints)
	writeSection("CURRENT CONCLUSION", s.CurrentConclusion)
	if s.CoreDiscussion != "" {
		sb.WriteString("CORE DISCUSSION\n")
		sb.WriteString(divider + "\n")
		sb.WriteString("  " + s.CoreDiscussion + "\n\n")
	}
	if len(s.DiscussionLog) > 0 {
		sb.WriteString("DISCUSSION LOG\n")
		sb.WriteString(divider + "\n")
		for _, d := range s.DiscussionLog {
			sb.WriteString(fmt.Sprintf("  [%s]  %s\n", d.Time, d.Conclusion))
			sb.WriteString("  " + d.Topic + "\n\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
