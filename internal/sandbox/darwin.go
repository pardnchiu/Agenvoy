//go:build darwin

package sandbox

import (
	"fmt"
)

// * if in macOS, always be true
func CheckDependence() error {
	return nil
}

func seatbeltProfile(home string) string {
	return fmt.Sprintf(`(version 1)
(deny default)
(allow process-exec)
(allow process-fork)
(allow sysctl-read)
(allow mach-lookup)
(allow signal)
(allow ipc-posix*)

;; read-only filesystem
(allow file-read*)

;; writable only under $HOME
(allow file-write*
    (subpath %q))

;; allow network
(allow network*)
`, home)
}

func Wrap(binary string, args []string, workDir string) (string, []string, error) {
	homeDir, err := vaildateDir(workDir)
	if err != nil {
		return "", nil, err
	}

	profile := seatbeltProfile(homeDir)

	sbArgs := []string{"-p", profile, binary}
	sbArgs = append(sbArgs, args...)

	return "sandbox-exec", sbArgs, nil
}
