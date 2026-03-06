package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/keychain"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run cmd/cli/main.go add")
		fmt.Println("  go run cmd/cli/main.go remove")
		fmt.Println("  go run cmd/cli/main.go list")
		fmt.Println("  go run cmd/cli/main.go list skills")
		fmt.Println("  go run cmd/cli/main.go run <input...>")
		fmt.Println("  go run cmd/cli/main.go run-allow <input...>")
		os.Exit(1)
	}

	if os.Args[1] == "add" {
		runAdd()
		return
	}

	if os.Args[1] == "remove" {
		runRemove()
		return
	}

	if os.Args[1] == "list" {
		if len(os.Args) > 2 && os.Args[2] == "skills" {
			scanner := skill.NewScanner()

			if len(scanner.Skills.ByName) == 0 {
				fmt.Println("No skills found")
				fmt.Println("\nScanned paths:")
				for _, path := range scanner.Skills.Paths {
					fmt.Printf("  - %s\n", path)
				}
				return
			}

			names := scanner.List()
			sort.Strings(names)

			fmt.Printf("Found %d skill(s):\n\n", len(names))
			for _, name := range names {
				s := scanner.Skills.ByName[name]
				fmt.Printf("• %s\n", name)
				if s.Description != "" {
					fmt.Printf("  %s\n", s.Description)
				}
				fmt.Printf("  Path: %s\n\n", s.Path)
			}
			return
		}

		cfg, err := keychain.Load()
		if err != nil {
			slog.Error("keychain.Load", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if len(cfg.Models) == 0 {
			fmt.Println("No models configured.")
			return
		}

		fmt.Printf("Found %d model(s):\n\n", len(cfg.Models))
		for _, m := range cfg.Models {
			fmt.Printf("• %s\n", m.Name)
			if m.Description != "" {
				fmt.Printf("  %s\n", m.Description)
			}
		}
		return
	}

	if os.Args[1] == "run" || os.Args[1] == "run-allow" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run cmd/cli/main.go run <input...>")
			fmt.Println("       go run cmd/cli/main.go run-allow <input...>")
			os.Exit(1)
		}

		allowAll := os.Args[1] == "run-allow"

		raw := strings.ReplaceAll(strings.Join(os.Args[2:], " "), `\n`, "\n")
		imagePattern := regexp.MustCompile(`--image\s+(\S+)`)
		filePattern := regexp.MustCompile(`--file\s+(\S+)`)
		var imageInputs []string
		for _, path := range imagePattern.FindAllStringSubmatch(raw, -1) {
			imageInputs = append(imageInputs, path[1])
		}
		var fileInputs []string
		for _, path := range filePattern.FindAllStringSubmatch(raw, -1) {
			fileInputs = append(fileInputs, path[1])
		}
		userInput := strings.TrimSpace(filePattern.ReplaceAllString(imagePattern.ReplaceAllString(raw, ""), ""))

		agentRegistry := getAgentRegistry()
		scanner := skill.NewScanner()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		selectorBot, err := copilot.New()
		if err != nil {
			slog.Error("failed to initialize", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if err := runEvents(ctx, cancel, func(ch chan<- agentTypes.Event) error {
			return exec.Run(ctx, selectorBot, agentRegistry, scanner, userInput, imageInputs, fileInputs, ch, allowAll)
		}); err != nil && ctx.Err() == nil {
			slog.Error("failed to execute", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}
}
