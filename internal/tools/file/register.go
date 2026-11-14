package file

import "github.com/pardnchiu/agenvoy/internal/tools/file/variant"

func Register() {
	registReadFiles()
	registFindFiles()
	registEditFile()
	registRemoveFile()
	registRestoreFile()
	registWriteResult()
	variant.Register()
}
