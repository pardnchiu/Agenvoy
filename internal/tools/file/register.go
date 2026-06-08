package file

import "github.com/pardnchiu/agenvoy/internal/tools/file/variant"

func Register() {
	registReadFile()
	registListFiles()
	registGlobFiles()
	registSearchFiles()
	registWriteFile()
	registPatchFile()
	variant.Register()
}
