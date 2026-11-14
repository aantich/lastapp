package control

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppHostCanonicalPorts(t *testing.T) {
	const app = "00000000-0000-4000-8000-000000000001"
	for _, tc := range []struct{ base, want string }{
		{"https://Apps.Example.test:443", "https://" + app + ".apps.example.test"},
		{"http://apps.example.test:80", "http://" + app + ".apps.example.test"},
		{"https://apps.example.test:8443", "https://" + app + ".apps.example.test:8443"},
	} {
		t.Run(tc.base, func(t *testing.T) {
			s := &Server{Config: Config{AppOrigin: tc.base}}
			if got := s.appHostedOrigin(app); got != tc.want {
				t.Fatalf("origin %q, want %q", got, tc.want)
			}
			for _, host := range []string{"invalid.apps.example.test", "invalid.apps.example.test:443", "invalid.apps.example.test:8443", "INVALID.APPS.EXAMPLE.TEST.:443"} {
				r := httptest.NewRequest("GET", "https://"+host+"/console/", nil)
				w := httptest.NewRecorder()
				if !s.serveAppHost(w, r) || w.Code != 404 {
					t.Fatalf("App namespace leaked to management for %q", host)
				}
			}
			if s.serveAppHost(httptest.NewRecorder(), httptest.NewRequest("GET", "https://console.example.test/console/", nil)) {
				t.Fatal("claimed unrelated console host")
			}
		})
	}
}

func TestAppHostingRejectsConsoleNamespaceOverlap(t *testing.T) {
	for _, tc := range []struct{ console, apps string }{
		{"https://console.example.test", "https://example.test"},
		{"https://console.apps.example.test", "https://apps.example.test"},
		{"https://CONSOLE.Apps.Example.test.:9443", "https://APPS.example.test:8443"},
		{"https://apps.example.test", "https://apps.example.test:8443"},
		{"https://00000000-0000-4000-8000-000000000001.apps.example.test", "https://apps.example.test"},
	} {
		t.Run(tc.console, func(t *testing.T) {
			// Reject before database setup, so even UUID-shaped console hosts can
			// never start inside the wildcard static namespace.
			s, e := Open(Config{PublicURL: tc.console, AppOrigin: tc.apps})
			if s != nil || e == nil || !strings.Contains(e.Error(), "host namespace") {
				t.Fatalf("overlapping host namespace was not rejected at startup: %v", e)
			}
		})
	}
	for _, console := range []string{"console.example.test", "apps.example.test.evil.test", "otherapps.example.test"} {
		s := &Server{Config: Config{AppOrigin: "https://apps.example.test"}}
		if s.serveAppHost(httptest.NewRecorder(), httptest.NewRequest("GET", "https://"+console+"/console/", nil)) {
			t.Fatalf("claimed a console outside the App namespace: %s", console)
		}
	}
}
