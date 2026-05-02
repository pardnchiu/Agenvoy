package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/common-nighthawk/go-figure"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

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
	if !go_pkg_filesystem_reader.Exists(filesystem.ConfigPath) {
		sb.WriteString("[gray]  (not found)[-]\n")
	} else if cfg, err := go_pkg_filesystem.ReadJSON[session.Config](filesystem.ConfigPath); err == nil {
		{
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
	}
	sb.WriteString(seperate + "\n\n")

	// * usage
	sb.WriteString(" USAGE\n")
	sb.WriteString(seperate + "\n")
	if !go_pkg_filesystem_reader.Exists(filesystem.UsagePath) {
		sb.WriteString("  (not found)\n")
	} else if usages, err := go_pkg_filesystem.ReadJSON[map[string]filesystem.Usage](filesystem.UsagePath); err == nil && len(usages) > 0 {
		{
			models := make([]string, 0, len(usages))
			for usage := range usages {
				models = append(models, usage)
			}
			sort.Strings(models)

			sb.WriteString(fmt.Sprintf("[gray]  %-32s %12s %12s[-]\n", "Model", "Input", "Output"))
			sb.WriteString("[gray]  " + strings.Repeat("─", 58) + "[-]\n")
			for _, model := range models {
				u := usages[model]
				label := model
				if len(model) > 32 {
					label = model[:31] + "…"
				}
				sb.WriteString(fmt.Sprintf("[gray]  %-32s %12s %12s[-]\n", label, utils.FormatInt(u.Input), utils.FormatInt(u.Output)))
			}
			sb.WriteString(seperate + "\n")

			hasCached := false
			for _, model := range models {
				u := usages[model]
				if u.CacheCreate > 0 || u.CacheRead > 0 {
					hasCached = true
					break
				}
			}
			if hasCached {
				sb.WriteString("\n CACHED USAGE\n")
				sb.WriteString(seperate + "\n")
				sb.WriteString(fmt.Sprintf("[gray]  %-32s %12s %12s[-]\n", "Model", "Create", "Read"))
				sb.WriteString("[gray]  " + strings.Repeat("─", 58) + "[-]\n")
				for _, model := range models {
					u := usages[model]
					if u.CacheCreate == 0 && u.CacheRead == 0 {
						continue
					}
					cacheCreate := "-"
					cacheRead := "-"
					if u.CacheCreate > 0 {
						cacheCreate = utils.FormatInt(u.CacheCreate)
					}
					if u.CacheRead > 0 {
						cacheRead = utils.FormatInt(u.CacheRead)
					}
					label := model
					if len(model) > 32 {
						label = model[:31] + "…"
					}
					sb.WriteString(fmt.Sprintf("[gray]  %-32s %12s %12s[-]\n", label, cacheCreate, cacheRead))
				}
			}
		}
	} else {
		sb.WriteString("[gray]  (format error)[-]\n")
	}
	sb.WriteString(seperate + "\n")

	return sb.String()
}
