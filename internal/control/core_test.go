package control

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

func TestRequiredSchemaVersionFollowsMigrations(t *testing.T) {
	goose.SetBaseFS(migrationFiles)
	all, e := goose.CollectMigrations("migrations", 0, goose.MaxVersion)
	if e != nil {
		t.Fatal(e)
	}
	last, e := all.Last()
	if e != nil {
		t.Fatal(e)
	}
	got, e := latestMigrationVersion()
	if e != nil || got != last.Version {
		t.Fatalf("required schema version %d (%v), goose latest %d", got, e, last.Version)
	}
}

func TestRequestValidation(t *testing.T) {
	for _, body := range []string{`{"unknown":1}`, `{} {}`, `{`} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		if httpapi.Decode(httptest.NewRecorder(), r, &struct {
			Name string `json:"name"`
		}{}) == nil {
			t.Fatalf("accepted invalid payload %s", body)
		}
	}
	for _, v := range []string{"", "bad", "00000000-0000-0000-0000-000000000000", "10000000000040008000000000000001"} {
		if registry.ID(v) == nil {
			t.Fatal("accepted noncanonical ID", v)
		}
	}
}
