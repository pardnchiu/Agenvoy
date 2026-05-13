package extensions

import "embed"

//go:embed apis/*.json
var APIs embed.FS
