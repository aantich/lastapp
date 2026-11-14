package admin

import "embed"

// Files contains the static administration application built by Vite.
//
//go:embed all:dist
var Files embed.FS

// PackageLock pins generated starter versions to the console's tested dependencies.
//
//go:embed package-lock.json
var PackageLock []byte
