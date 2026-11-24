package hook

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type Script struct {
	Path    string
	WorkDir string
	Runtime string
}

var (
	hookRuntimes = map[string]string{
		".py": "python3",
		".js": "node",
	}
	hookStateKey = [...]string{
		BeforeUserSend:   "before_user_send",
		BeforeToolCall:   "before_tool_call",
		AfterToolCall:    "after_tool_call",
		AfterToolError:   "after_tool_error",
		AfterRequest:     "after_request",
		AfterTaskDone:    "after_task_done",
		BeforeTaskCancel: "before_task_cancel",
		AfterTaskCancel:  "after_task_cancel",
	}
)

func Load(workDir string) map[string]Script {
	dirs := []string{filesystem.HooksDir}
	if strings.TrimSpace(workDir) == "" {
		workDir = filesystem.HomeDir
	} else {
		dirs = append(dirs, filesystem.WorkHooksDir(workDir))
	}

	dic := map[string]Script{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			ext := filepath.Ext(name)
			key := strings.TrimSuffix(name, ext)
			runtime, ok := hookRuntimes[ext]
			// NOTE: 只讀規範的 hookStateKey 檔案
			if !ok || !slices.Contains(hookStateKey[:], key) || entry.IsDir() {
				continue
			}
			// NOTE: 如果 py 版本已存在 dic 則跳過 (py > js)
			if script, ok := dic[key]; ok && filepath.Dir(script.Path) == dir && script.Runtime == "python3" {
				continue
			}
			dic[key] = Script{
				Path:    filepath.Join(dir, name),
				WorkDir: workDir,
				Runtime: runtime,
			}
		}
	}
	return dic
}

func (s State) String() string {
	parts := strings.Split(hookStateKey[s], "_")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
