package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MCP tools call the same operations as console handlers. The former
// MCP→HTTP loopback (a synthetic request and recorded response around a
// console handler, with a synthesized If-Match) must not return.
func TestNoHTTPLoopback(t *testing.T) {
	files, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range files {
		if file == "loopback_test.go" {
			continue
		}
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		for _, symbol := range []string{"executeManagement", "operationResponse", "httptest.NewRecorder"} {
			if strings.Contains(string(b), symbol) && !(symbol == "httptest.NewRecorder" && strings.HasSuffix(file, "_test.go")) {
				t.Fatalf("%s references the removed MCP→HTTP loopback (%s); call the operation directly", file, symbol)
			}
		}
	}
}
