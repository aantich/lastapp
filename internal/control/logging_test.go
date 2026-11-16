package control

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{mux: http.NewServeMux(), Log: slog.New(slog.NewJSONHandler(&buf, nil))}
	h := s.Handler()
	secret := "s3cret-value"
	r := httptest.NewRequest("GET", "/v1/config?token="+secret, nil)
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("Cookie", "lastapp_session="+secret)
	r.Header.Set("X-Request-ID", "client-supplied")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/missing?password="+secret, nil))
	out := buf.String()
	if strings.Contains(out, secret) || strings.Contains(out, "client-supplied") {
		t.Fatalf("access log leaks request data: %s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one access line per request, got %d: %s", len(lines), out)
	}
	var first, second map[string]any
	if json.Unmarshal([]byte(lines[0]), &first) != nil || json.Unmarshal([]byte(lines[1]), &second) != nil {
		t.Fatal("access log is not structured")
	}
	if first["msg"] != "http request" || first["route"] != "GET /v1/config" || first["method"] != "GET" || first["status"] != float64(200) || first["request_id"] != w.Header().Get("X-Request-ID") || first["request_id"] == "" {
		t.Fatalf("unexpected access line: %v", first)
	}
	if second["status"] != float64(404) || second["request_id"] == first["request_id"] {
		t.Fatalf("unexpected access line: %v", second)
	}
}
