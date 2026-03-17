package keychain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func Get(key string) string {
	if val := readKeychain(key); val != "" {
		return val
	}
	return os.Getenv(key)
}

func Set(key, value string) error {
	if value == "" {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		return setSecretOnMac(key, value)
	default:
		if ok := setSecret(key, value); ok == nil {
			return nil
		}
		return setFallback(key, value)
	}
}

func setSecret(key, value string) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", "agenvoy/"+key,
		"service", "agenvoy", "account", key)
	cmd.Stdin = strings.NewReader(value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool store: %s", out)
	}
	return nil
}

func getSecret(key string) string {
	out, err := exec.Command("secret-tool", "lookup",
		"service", "agenvoy", "account", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func setSecretOnMac(key, value string) error {
	exec.Command("security", "delete-generic-password",
		"-s", "agenvoy",
		"-a", key).Run()

	cmd := exec.Command("security", "add-generic-password",
		"-s", "agenvoy",
		"-a", key,
		"-w", value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-generic-password: %s", out)
	}
	return nil
}

func getSecretFromMac(key string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-s", "agenvoy",
		"-a", key,
		"-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func setFallback(key, value string) error {
	// configData, err := utils.GetConfigDir()
	// if err != nil {
	// 	return fmt.Errorf("utils.GetConfigDir: %w", err)
	// }

	path := filepath.Join(filesystem.AgenvoyDir, ".secrets")
	lines := readFallbackLines()
	prefix := key + "="
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}
	data := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return filesystem.WriteFile(path, data, 0600)
}

func getFallback(key string) string {
	prefix := key + "="
	for _, l := range readFallbackLines() {
		if v, ok := strings.CutPrefix(l, prefix); ok {
			return v
		}
	}
	return ""
}

func readFallbackLines() []string {
	// configData, err := utils.GetConfigDir()
	// if err != nil {
	// 	return nil
	// }

	path := filepath.Join(filesystem.AgenvoyDir, ".secrets")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func readKeychain(key string) string {
	switch runtime.GOOS {
	case "darwin":
		return getSecretFromMac(key)
	default:
		if secret := getSecret(key); secret != "" {
			return secret
		}
		return getFallback(key)
	}
}
