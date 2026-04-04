//go:build darwin

package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// * if in macOS, always be true
func CheckDependence() error {
	return nil
}

func seatbeltProfile(home string) string {
	deniedDirs, deniedFiles := deniedPaths(home)

	var deny strings.Builder
	for _, d := range deniedDirs {
		fmt.Fprintf(&deny, "(deny file-read* (subpath %q))\n", d)
		fmt.Fprintf(&deny, "(deny file-write* (subpath %q))\n", d)
	}
	for _, f := range deniedFiles {
		fmt.Fprintf(&deny, "(deny file-read* (literal %q))\n", f)
		fmt.Fprintf(&deny, "(deny file-write* (literal %q))\n", f)
	}

	keychainDir := filepath.Join(home, "Library", "Keychains")

	return fmt.Sprintf(`(version 1)
(deny default)
(allow process-exec)
(allow process-fork)
(allow sysctl-read)
(allow mach-lookup)
(allow signal)
(allow ipc-posix*)

;; deny sensitive paths
%s
;; re-allow keychain access (required for keyring/Security framework)
(allow file-read* (subpath %q))
(allow file-write* (subpath %q))

;; read-only filesystem
(allow file-read*)

;; writable only under $HOME
(allow file-write*
    (subpath %q))

;; allow /dev access (required for /dev/null, /dev/random, etc.)
(allow file-read* (subpath "/dev"))
(allow file-write* (subpath "/dev"))

;; allow network
(allow network*)
`, deny.String(), keychainDir, keychainDir, home)
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
