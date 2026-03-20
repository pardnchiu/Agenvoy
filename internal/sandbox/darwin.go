//go:build darwin

package sandbox

import (
	"fmt"
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
;; read-only filesystem
(allow file-read*)

;; writable only under $HOME
(allow file-write*
    (subpath %q))

;; allow network
(allow network*)
`, deny.String(), home)
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
