package allowSkill

import "strings"

func Match(workDir, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if LoadGlobal()[name] {
		return true
	}
	if strings.TrimSpace(workDir) != "" && LoadProject(workDir)[name] {
		return true
	}
	return false
}
