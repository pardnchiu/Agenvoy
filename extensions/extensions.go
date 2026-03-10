package extensions

import "embed"

//go:embed apis/*.json
var APIs embed.FS

//go:embed skills/*/SKILL.md
var Skills embed.FS
