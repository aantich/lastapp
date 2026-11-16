package op

import (
	"context"
	"time"
)

// RequestTimeout bounds the database work of one console API or MCP request.
var RequestTimeout = 25 * time.Second

type requestBaseKey struct{}

// WithRequestDeadline starts a request's deadline and remembers the request
// context without it, so renewDeadline can give a later phase a fresh one.
func WithRequestDeadline(base context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(base, RequestTimeout)
	return context.WithValue(ctx, requestBaseKey{}, base), cancel
}

// RenewDeadline gives the database phase after a PBKDF2 derivation its own
// request deadline. The derivation cannot observe a context; its cost is
// bounded by passwordSlots and maxPasswordInputBytes. A deadline spanning it
// only turned a completed derivation into a 500 when the CPU was slow (for
// example under the race detector on a loaded test machine). The renewed
// context keeps ctx's values and the request's own cancellation.
func RenewDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	base, ok := ctx.Value(requestBaseKey{}).(context.Context)
	if !ok {
		return ctx, func() {}
	}
	next, cancel := context.WithTimeout(context.WithoutCancel(ctx), RequestTimeout)
	stop := context.AfterFunc(base, cancel)
	return next, func() { stop(); cancel() }
}
