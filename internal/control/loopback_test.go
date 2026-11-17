package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// operationPackages are the transport-independent packages below control.
var operationPackages = []string{"registry", "access", "op", "config", "databases", "roles", "membership", "inventory", "entities", "publisher", "delivery", "worker"}

// transportIndependentFiles are the files of the HTTP-facing data API package
// (internal/dataapi) that hold its non-HTTP logic: SQL builders, the definition
// cache and Tenant fairness. They stay free of net/http like the operations.
var transportIndependentFiles = []string{"cache.go", "fairness.go", "sql.go"}

// MCP tools call the same operations as console handlers. The former
// MCP→HTTP loopback (a synthetic request and recorded response around a
// console handler, with a synthesized If-Match) must not return, here or in
// the operation packages, and those packages never see an HTTP request.
func TestNoHTTPLoopback(t *testing.T) {
	files, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, pkg := range operationPackages {
		more, e := filepath.Glob(filepath.Join("..", pkg, "*.go"))
		if e != nil || len(more) == 0 {
			t.Fatal("operation package", pkg, "not found", e)
		}
		for _, file := range more {
			b, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			for _, symbol := range []string{`"net/http"`, `"github.com/aantich/lastapp/internal/control"`} {
				if strings.Contains(string(b), symbol) {
					t.Fatalf("%s imports %s; operations stay transport-independent", file, symbol)
				}
			}
		}
		files = append(files, more...)
	}
	for _, name := range transportIndependentFiles {
		file := filepath.Join("..", "dataapi", name)
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		for _, symbol := range []string{`"net/http"`, `"github.com/aantich/lastapp/internal/control"`} {
			if strings.Contains(string(b), symbol) {
				t.Fatalf("%s imports %s; data API SQL and caches stay transport-independent", file, symbol)
			}
		}
	}
	// The HTTP-facing packages below control serve requests themselves; they
	// must not route anything back through a recorded console handler either.
	// The agent package holds the MCP tool adapters, which call operations
	// directly instead of the former loopback.
	for _, pkg := range []string{"agent", "dataapi", "session"} {
		more, e := filepath.Glob(filepath.Join("..", pkg, "*.go"))
		if e != nil || len(more) == 0 {
			t.Fatal("HTTP-facing package", pkg, "not found", e)
		}
		for _, file := range more {
			b, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			if strings.Contains(string(b), `"github.com/aantich/lastapp/internal/control"`) {
				t.Fatalf("%s imports internal/control; HTTP-facing packages sit below it", file)
			}
		}
		files = append(files, more...)
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
