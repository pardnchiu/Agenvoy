package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func executeMessage(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	runInSuspend("> "+input, exec.Command("make", "cli", input))
}

func executeCommand(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	runInSuspend("$ "+input, exec.Command("sh", "-c", input))
}

func runInSuspend(banner string, cmd *exec.Cmd) {
	app.Suspend(func() {
		fmt.Print("\033[H\033[2J" + banner + "\n")

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()

		stty := exec.Command("stty", "sane")
		stty.Stdin = os.Stdin
		stty.Run()

		fmt.Print("\n[Press Shift+Q to return]")

		wait := exec.Command("bash", "-c", `while true; do read -r -s -n1 key; [[ "$key" == "Q" ]] && break; done`)
		wait.Stdin = os.Stdin
		wait.Stdout = os.Stdout
		wait.Run()

		fmt.Print("\033c")
		time.Sleep(50 * time.Millisecond)
	})

	app.Sync()
}
