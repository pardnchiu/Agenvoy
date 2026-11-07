package webapp

import (
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

func appURL() string {
	return "http://127.0.0.1:" + filesystem.Port
}

func appVersion() string {
	if runtime.IsDev() {
		return "0.0.0"
	}
	return strings.TrimPrefix(runtime.CurrentVersion, "v")
}
