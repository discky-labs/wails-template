package fixtures

import "embed"

// ruleid: go-wails-embed-missing-all-prefix
//go:embed frontend/dist
var bad embed.FS

// ok: go-wails-embed-missing-all-prefix
//go:embed all:frontend/dist
var good embed.FS
