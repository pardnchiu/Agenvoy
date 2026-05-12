package cli

import (
	"log/slog"
	"os"

	"github.com/manifoldco/promptui"
)

func Pick(label string, items []string) string {
	labels := make([]string, len(items)+1)
	copy(labels, items)
	labels[len(items)] = "exit"

	sel := promptui.Select{
		Label:        label,
		Items:        labels,
		HideSelected: true,
	}
	idx, _, err := sel.Run()
	if err != nil {
		slog.Error("promptui.Select.Run", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if idx == len(items) {
		os.Exit(0)
	}
	return items[idx]
}
