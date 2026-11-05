package boundary

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type pathSet struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

var cache sync.Map

func get(sessionID string) *pathSet {
	if v, ok := cache.Load(sessionID); ok {
		return v.(*pathSet)
	}
	ps := &pathSet{paths: map[string]struct{}{}}
	actual, _ := cache.LoadOrStore(sessionID, ps)
	return actual.(*pathSet)
}

func Grant(sessionID string, list ...string) {
	if len(list) == 0 {
		return
	}
	ps := get(sessionID)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, one := range list {
		one = filepath.Clean(strings.TrimSpace(one))
		if one == "" || one == "." || one == string(filepath.Separator) {
			continue
		}
		ps.paths[one] = struct{}{}
	}
}

func isGranted(sessionID, absPath string) bool {
	v, ok := cache.Load(sessionID)
	if !ok {
		return false
	}
	ps := v.(*pathSet)
	ps.mu.Lock()
	defer ps.mu.Unlock()
	_, hit := ps.paths[filepath.Clean(absPath)]
	return hit
}

func resolveRaw(baseDir, path string) (string, error) {
	abs := go_pkg_utils.AbsPath(baseDir, path)
	if abs == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "path", "empty with no base directory")
	}
	return go_pkg_filesystem.RealPath(abs)
}

func deniedErr(absPath string) error {
	return utilsmsg.New(utilsmsg.Denied, "path", "%s is on denied_path and is permanently off limits for both reads and writes; it cannot be approved and retrying the same path fails the same way", absPath)
}

func Resolve(sessionID, baseDir, path string) (string, error) {
	abs, homeErr := go_pkg_filesystem.AbsPath(baseDir, path, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
	if homeErr == nil && IsDeniedPath(abs) {
		return "", deniedErr(abs)
	}
	if homeErr == nil && !IsSensitivePath(abs) {
		return abs, nil
	}

	restricted, rawErr := resolveRaw(baseDir, path)
	if rawErr != nil {
		if homeErr != nil {
			return "", homeErr
		}
		return "", rawErr
	}
	if IsDeniedPath(restricted) {
		return "", deniedErr(restricted)
	}
	if isGranted(sessionID, restricted) {
		return restricted, nil
	}
	if homeErr != nil {
		return "", utilsmsg.New(utilsmsg.Denied, "path", "%w; the user was not asked or did not approve this path, so retrying the same path fails the same way", homeErr)
	}
	return "", utilsmsg.New(utilsmsg.Denied, "path", "%s holds credentials or key material; the user was not asked or did not approve this path, so retrying the same path fails the same way", restricted)
}

func Restricted(sessionID, workDir, toolName, toolArgs string) []string {
	list := rawPaths(toolName, toolArgs)
	if len(list) == 0 {
		return nil
	}

	base := baseDir(toolName, workDir)
	seen := map[string]bool{}
	var out []string
	for _, one := range list {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}
		abs, err := resolveRaw(base, one)
		if err != nil {
			continue
		}
		if _, homeErr := go_pkg_filesystem.AbsPath(base, one, go_pkg_filesystem.AbsPathOption{HomeOnly: true}); homeErr == nil && !IsSensitivePath(abs) {
			continue
		}
		if seen[abs] || IsDeniedPath(abs) || isGranted(sessionID, abs) {
			continue
		}

		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

func WriteBinds(sessionID, baseDir string, list []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, one := range list {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}
		if !filepath.IsAbs(one) && !strings.HasPrefix(one, "~") {
			return nil, utilsmsg.New(utilsmsg.Invalid, "write_paths", "needs absolute paths, got %q", one)
		}
		if home, err := go_pkg_filesystem.AbsPath(baseDir, one, go_pkg_filesystem.AbsPathOption{HomeOnly: true}); err == nil {
			if IsDeniedPath(home) {
				return nil, deniedErr(home)
			}
			continue
		}

		abs, err := resolveRaw(baseDir, one)
		if err != nil {
			return nil, utilsmsg.New(utilsmsg.Invalid, "write_paths", "%q: %w", one, err)
		}
		if IsDeniedPath(abs) {
			return nil, deniedErr(abs)
		}
		if !isGranted(sessionID, abs) {
			return nil, utilsmsg.New(utilsmsg.Denied, "write_paths", "write path %s was not approved — it has to be approved in this call's prompt; retrying without that changes nothing", abs)
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out, nil
}

func baseDir(toolName, workDir string) string {
	if toolName == "fetch_page" || strings.TrimSpace(workDir) == "" {
		return filesystem.DownloadDir
	}
	return workDir
}

func rawPaths(toolName, toolArgs string) []string {
	switch toolName {
	case "read_files":
		var p struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		out := make([]string, 0, len(p.Files))
		for _, f := range p.Files {
			out = append(out, f.Path)
		}
		return out

	case "edit_file", "open_file":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		return []string{p.Path}

	case "find_files":
		var p struct {
			Queries []struct {
				Dir string `json:"dir"`
			} `json:"queries"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		out := make([]string, 0, len(p.Queries))
		for _, q := range p.Queries {
			out = append(out, q.Dir)
		}
		return out

	case "file_history":
		var p struct {
			Path  string   `json:"path"`
			Paths []string `json:"paths"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		return append([]string{p.Path}, p.Paths...)

	case "run_command", "run_command_background":
		var p struct {
			WritePaths []string `json:"write_paths"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil {
			return nil
		}
		return p.WritePaths

	case "fetch_page":
		var p struct {
			Save   bool   `json:"save"`
			SaveTo string `json:"save_to"`
		}
		if json.Unmarshal([]byte(toolArgs), &p) != nil || !p.Save {
			return nil
		}
		return []string{p.SaveTo}
	}
	return nil
}
