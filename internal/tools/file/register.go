package file

import "github.com/pardnchiu/agenvoy/internal/tools/file/variant"

func Register() {
	registReadFiles()
	registFindFiles()
	registWriteFile()
	registRestoreFile()
	registWriteResult()
	variant.Register()
}
