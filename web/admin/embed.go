package admin

import "embed"

// Files contains the static administration application built by Vite.
//go:embed all:dist
var Files embed.FS
