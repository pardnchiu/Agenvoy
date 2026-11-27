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
		BeforeUserSend:  "before_user_send",
		BeforeToolCall:  "before_tool_call",
		AfterToolCall:   "after_tool_call",
		AfterToolError:  "after_tool_error",
		AfterAskUser:    "after_ask_user",
		AfterTaskDone:   "after_task_done",
		AfterTaskCancel: "after_task_cancel",
	}
)

func Load(workDir string) map[string]Script {
	if strings.TrimSpace(workDir) == "" {
		workDir = filesystem.HomeDir
	}

	dic := map[string]Script{}
	entries, err := os.ReadDir(filesystem.HooksDir)
	if err != nil {
		return dic
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
		if script, ok := dic[key]; ok && script.Runtime == "python3" {
			continue
		}
		dic[key] = Script{
			Path:    filepath.Join(filesystem.HooksDir, name),
			WorkDir: workDir,
			Runtime: runtime,
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

func Names() []string {
	return slices.Clone(hookStateKey[:])
}
