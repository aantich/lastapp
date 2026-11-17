package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aantich/lastapp/internal/httpapi"
	"github.com/aantich/lastapp/internal/registry"
)

func TestPasswordLoginProxyBudgets(t *testing.T) {
	s, c, _, _, _ := foundationFixture(t)
	var e error
	s.trustedProxies, e = httpapi.ParseTrustedProxies([]string{"10.0.0.1"})
	if e != nil {
		t.Fatal(e)
	}
	login := func(peer, forwarded, email string) int {
		// Length rejection follows rate charging. Avoid slow PBKDF2 work so the
		// burst assertions remain independent of machine speed and race overhead.
		b, _ := json.Marshal(registry.Row{"email": email, "password": strings.Repeat("x", 1025)})
		r := httptest.NewRequest("POST", "/auth/login/password", bytes.NewReader(b))
		r.RemoteAddr = peer
		r.Header.Set("Origin", s.Config.PublicURL)
		r.Header.Set("X-Forwarded-For", forwarded)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 1; i <= 13; i++ {
		if got := login("10.0.0.1:1234", fmt.Sprintf("198.51.100.%d", i), fmt.Sprintf("client-%d@example.test", i)); got != 401 {
			t.Fatalf("client %d: %d", i, got)
		}
	}
	// Rotating direct spoofed forwarding headers cannot evade a peer's budget.
	for i := 1; i <= 13; i++ {
		want := 401
		if i == 13 {
			want = 429
		}
		if got := login("203.0.113.1:1234", fmt.Sprintf("192.0.2.%d", i), fmt.Sprintf("spoof-%d@example.test", i)); got != want {
			t.Fatalf("spoof %d: got %d want %d", i, got, want)
		}
	}
	// A distributed attack against one email still spends its shared email budget.
	for i := 1; i <= 9; i++ {
		want := 401
		if i == 9 {
			want = 429
		}
		if got := login("10.0.0.1:1234", fmt.Sprintf("192.0.2.%d", i), "target@example.test"); got != want {
			t.Fatalf("email attempt %d: got %d want %d", i, got, want)
		}
	}
}
