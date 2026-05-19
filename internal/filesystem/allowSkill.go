package filesystem

import "path/filepath"

const (
	AllowSkillRelPath  = ".agenvoy/allow_skill"
	AllowSkillFileName = "allow_skill"
)

func AllowSkillProjectPath(workDir string) string {
	return filepath.Join(workDir, AllowSkillRelPath)
}

func AllowSkillProjectDir(workDir string) string {
	return filepath.Join(workDir, filepath.Dir(AllowSkillRelPath))
}

func AllowSkillGlobalPath() string {
	return filepath.Join(AgenvoyDir, AllowSkillFileName)
}
