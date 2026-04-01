package extensions

import "embed"

//go:embed apis/*.json
var APIs embed.FS

//go:embed skills
var Skills embed.FS
