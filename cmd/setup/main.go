package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
)

const installURL = "https://agenvoy.com/scripts/install.sh"

func main() {
	if p := findAgen(); p != "" {
		launch(p)
	}

	fmt.Println("==> Installing Agenvoy...")
	if err := install(); err != nil {
		die(fmt.Sprintf("installation failed: %v", err))
	}

	if p := findAgen(); p != "" {
		launch(p)
	}
	die("agen not found after installation. Open a new terminal and run: agen")
}

func findAgen() string {
	if p, err := exec.LookPath("agen"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/local/bin/agen", os.Getenv("HOME") + "/.local/bin/agen"} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func install() error {
	resp, err := http.Get(installURL)
	if err != nil {
		return fmt.Errorf("download install.sh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download install.sh: HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "agenvoy-install-*.sh")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("write install script: %w", err)
	}
	f.Close()

	cmd := exec.Command("bash", tmp)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launch(path string) {
	fmt.Println(" ok Launching Agenvoy...")
	err := syscall.Exec(path, []string{"agen"}, os.Environ())
	if err != nil {
		cmd := exec.Command(path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	os.Exit(0)
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, " xx %s\n", msg)
	if fi, _ := os.Stdin.Stat(); fi != nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Print("\nPress Enter to close... ")
		b := make([]byte, 1)
		os.Stdin.Read(b)
	}
	os.Exit(1)
}
