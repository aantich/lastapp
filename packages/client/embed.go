// Package clientsdk exposes the authoritative browser SDK to remote authors.
package clientsdk

import _ "embed"

//go:embed src/index.ts
var Source string
