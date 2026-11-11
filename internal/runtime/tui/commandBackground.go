package tui

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func backgroundCancelPopup(sessionID string) *Popup {
	legs := interactive.ListRunningBackground(sessionID)
	if len(legs) == 0 {
		return nil
	}
	options := []string{"No"}
	values := []string{""}
	for i, leg := range legs {
		options = append(options, leg.Description+"  "+leg.ID)
		values = append(values, strconv.Itoa(i))
	}
	if len(legs) > 1 {
		options = append(options, "All")
		values = append(values, "all")
	}
	return &Popup{
		kind:     popupSingleSelect,
		title:    "Cancel background task?",
		subtitle: fmt.Sprintf("%d running", len(legs)),
		options:  options,
		values:   values,
		onConfirm: func(chosen string) any {
			pick := BackgroundCancelPick{session: sessionID}
			switch chosen {
			case "":
			case "all":
				pick.legs = legs
			default:
				if i, err := strconv.Atoi(chosen); err == nil && i < len(legs) {
					pick.legs = legs[i : i+1]
				}
			}
			return pick
		},
	}
}

func (t TUI) openBackgroundCancelConfirm(pick BackgroundCancelPick) (TUI, tea.Cmd) {
	subtitle := fmt.Sprintf("all %d running background tasks", len(pick.legs))
	if len(pick.legs) == 1 {
		subtitle = pick.legs[0].Description + "  " + pick.legs[0].ID
	}
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "Are you sure? Its output will be discarded.",
		subtitle: subtitle,
		options:  []string{"No", "Yes  cancel it"},
		values:   []string{"no", "yes"},
		onConfirm: func(chosen string) any {
			if chosen != "yes" {
				return nil
			}
			pick.confirmed = true
			return pick
		},
	}
	return t, nil
}

func cancelBackground(pick BackgroundCancelPick) tea.Cmd {
	client := ipcClient.Load()
	if client == nil {
		return notice(msgWarn("daemon offline") + "\n")
	}
	for _, leg := range pick.legs {
		if err := client.CancelBackground(pick.session, leg.JobID, leg.ID); err != nil {
			return notice(msgError(err.Error()) + "\n")
		}
	}
	return notice(msgLog(fmt.Sprintf("canceled %d background task(s)", len(pick.legs))) + "\n")
}

type backgroundPoll struct{}

func pollBackground() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return backgroundPoll{} })
}

func (t TUI) refreshBackground() (TUI, tea.Cmd) {
	idle := !t.running && !t.connecting && t.backgroundRunning == 0
	t.backgroundRunning = len(interactive.ListRunningBackground(t.currentSessionID))
	if idle && t.backgroundRunning > 0 {
		return t, tea.Batch(pollBackground(), t.spinner.Tick)
	}
	return t, pollBackground()
}
