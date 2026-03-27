package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func setDefault() string {
	_, _, width, _ := contentView.GetInnerRect()
	seperate := strings.Repeat("─", width/2)

	var sb strings.Builder

	ascii := figure.NewFigure("Agenvoy", "ogre", true)
	sb.WriteString(ascii.String())

	// * description
	metaMu.RLock()
	description := metaDescription
	release := metaRelease
	commits := metaCommits
	metaMu.RUnlock()
	if release == "" {
		release = "v*.*.*"
	}

	sb.WriteString(seperate + "\n")
	sb.WriteString(description + "\n\n")
	sb.WriteString(fmt.Sprintf("%-12s: ", "Version") + projectVersion + "\n")
	if release != "v-.-.-" && release != projectVersion {
		sb.WriteString(fmt.Sprintf("%-12s: ", "Last Version") + "[green]" + release + "[-]\n")
	} else {
		sb.WriteString(fmt.Sprintf("%-12s: ", "Last Version") + release + "\n")
	}
	sb.WriteString(fmt.Sprintf("%-12s: ", "Developer") + "Pardn Chiu[gray] － dev@pardn.io[-]\n")
	sb.WriteString(seperate + "\n\n")

	// * commits
	sb.WriteString(" COMMITS\n")
	sb.WriteString(seperate + "\n")
	if len(commits) > 0 {
		for _, c := range commits {
			sb.WriteString(fmt.Sprintf("[gray]  %s: %s[-]\n", c.Date, c.Message))
		}
	} else {
		sb.WriteString("[gray]  ...[-]\n")
	}
	sb.WriteString(seperate + "\n\n")

	// * config
	sb.WriteString(" CONFIG\n")
	sb.WriteString(seperate + "\n")
	if bytes, err := os.ReadFile(filesystem.ConfigPath); err == nil {
		var cfg session.Config
		if json.Unmarshal(bytes, &cfg) == nil {
			if cfg.SessionID != "" {
				sb.WriteString(fmt.Sprintf("[gray]  %-9s: %s[-]\n", "SESSION", cfg.SessionID))
			}
			if cfg.PlannerModel != "" {
				sb.WriteString(fmt.Sprintf("[gray]  %-9s: %s[-]\n", "PLANNER", cfg.PlannerModel))
			}
			if cfg.ReasoningLevel != "" {
				sb.WriteString(fmt.Sprintf("[gray]  %-9s: %s[-]\n", "REASONING", cfg.ReasoningLevel))
			}
			if len(cfg.Models) > 0 {
				sb.WriteString("\n")
				sb.WriteString("[gray]  MODELs:[-]\n")
				for _, m := range cfg.Models {
					sb.WriteString(fmt.Sprintf("[gray]    - %s[-]\n", m.Name))
				}
			}
			if len(cfg.Compats) > 0 {
				sb.WriteString("\n")
				sb.WriteString("[gray]  COMPACTs:[-]\n")
				for _, c := range cfg.Compats {
					sb.WriteString(fmt.Sprintf("[gray]    - %s: %s[-]\n", c.Provider, c.URL))
				}
			}
			if len(cfg.Keys) > 0 {
				sb.WriteString("\n")
				sb.WriteString("[gray]  KEYs:[-]\n")
				for _, k := range cfg.Keys {
					sb.WriteString(fmt.Sprintf("[gray]    - %s[-]\n", k))
				}
			}
		}
	} else {
		sb.WriteString("[gray]  (not found)[-]\n")
	}
	sb.WriteString(seperate + "\n\n")

	// * usage
	sb.WriteString(" USAGE\n")
	sb.WriteString(seperate + "\n")
	if data, err := os.ReadFile(filesystem.UsagePath); err == nil {
		var usages map[string]filesystem.Usage
		if json.Unmarshal(data, &usages) == nil && len(usages) > 0 {
			models := make([]string, 0, len(usages))
			for usage := range usages {
				models = append(models, usage)
			}
			sort.Strings(models)
			sb.WriteString(fmt.Sprintf("[gray]  %-40s %16s %16s[-]\n", "Model", "Input", "Output"))
			sb.WriteString("[gray]  " + strings.Repeat("─", 74) + "[-]\n")
			for _, model := range models {
				u := usages[model]
				sb.WriteString(fmt.Sprintf("[gray]  %-40s %16s %16s[-]\n", model, utils.FormatInt(u.Input), utils.FormatInt(u.Output)))
			}
		} else {
			sb.WriteString("[gray]  (format error)[-]\n")
		}
	} else {
		sb.WriteString("  (not found)\n")
	}
	sb.WriteString("\n" + seperate + "\n")

	return sb.String()
}
