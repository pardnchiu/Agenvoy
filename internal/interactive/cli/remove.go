package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func RunRemove() {
	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if len(cfg.Models) == 0 {
		fmt.Println("No models configured.")
		return
	}

	items := make([]string, 0, len(cfg.Models)+1)
	for _, m := range cfg.Models {
		if m.Description != "" {
			items = append(items, fmt.Sprintf("%s (%s)", m.Name, m.Description))
		} else {
			items = append(items, m.Name)
		}
	}
	items = append(items, "exit")

	selector := promptui.Select{
		Label:        "Select model to remove",
		Items:        items,
		HideSelected: true,
	}
	index, _, err := selector.Run()
	if err != nil {
		slog.Error("selector.Run", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if index == len(cfg.Models) {
		return
	}

	removed := cfg.Models[index]
	cfg.Models = append(cfg.Models[:index], cfg.Models[index+1:]...)

	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save", slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] %q removed\n", removed.Name)
}
