package httpapi

import (
	"context"
	"net/http"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// Endpoint is a console handler for an authenticated principal.
type Endpoint func(http.ResponseWriter, *http.Request, access.Principal) error

// Session is the console-session wrapper: request deadline, page context,
// authentication, request limits, the initial-password gate and CSRF for
// writes, then the endpoint, with every error sent by WriteError.
type Session struct {
	Principal func(context.Context, *http.Request) (access.Principal, error)
	Limit     func(http.ResponseWriter, access.Principal, *http.Request) error
	CSRF      func(context.Context, *http.Request) error
}

// Handle wraps fn with the session checks.
func (s Session) Handle(fn Endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := op.WithRequestDeadline(r.Context())
		defer cancel()
		ctx = WithPageInfo(ctx)
		r = r.WithContext(ctx)
		p, e := s.Principal(ctx, r)
		if e == nil {
			e = s.Limit(w, p, r)
		}
		if e == nil && p.PasswordChangeRequired && r.URL.Path != "/v1/me" && r.URL.Path != "/auth/password" && r.URL.Path != "/auth/logout" {
			e = registry.Problem(403, "password_change_required", "Change your initial password before using this account.")
		}
		if e == nil && r.Method != "GET" && r.Method != "HEAD" {
			e = s.CSRF(ctx, r)
		}
		if e == nil {
			e = fn(w, r, p)
		}
		if e != nil {
			WriteError(w, r, e)
		}
	}
}
