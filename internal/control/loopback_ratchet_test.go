package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxLoopbackReferences is the number of remaining references to the MCP→HTTP
// loopback executeManagement (definition, call sites, comments and tests).
// MCP tools must call operations directly; lower this as domains migrate and
// never raise it. It reaches 0 when the loopback is deleted.
const maxLoopbackReferences = 20

func TestLoopbackRatchet(t *testing.T) {
	files, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, file := range files {
		if file == "loopback_ratchet_test.go" {
			continue
		}
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		n += strings.Count(string(b), "executeManagement")
	}
	if n > maxLoopbackReferences {
		t.Fatalf("executeManagement references grew to %d (limit %d); call the operation directly instead", n, maxLoopbackReferences)
	}
	if n < maxLoopbackReferences {
		t.Fatalf("executeManagement references fell to %d; lower maxLoopbackReferences to lock in the progress", n)
	}
}
