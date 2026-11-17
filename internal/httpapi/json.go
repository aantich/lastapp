package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/op"
	"github.com/aantich/lastapp/internal/registry"
)

// Decode reads exactly one bounded JSON object without unknown fields.
func Decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return registry.Bad("Invalid or unsupported JSON request fields.")
	}
	var extra any
	if e := d.Decode(&extra); !errors.Is(e, io.EOF) {
		return registry.Bad("Request must contain one JSON object.")
	}
	return nil
}

// Send writes value as a JSON response with status.
func Send(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Headers are already sent; a failed write means the client went away.
	_ = json.NewEncoder(w).Encode(value)
}

// WriteError is the single HTTP error writer: op.Classify maps err to its
// public payload, and unmapped errors are logged only as diagnostics.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	o := op.Classify(err)
	if o.Unmapped {
		fields := []any{"operation", r.Method + " " + r.Pattern}
		if rid := r.Header.Get("X-Request-ID"); rid != RequestID(r.Context()) {
			// MCP tool calls run operations under their own request IDs.
			fields = append(fields, "operation_request_id", rid)
		}
		RequestLogger(r.Context()).Error("request failed", append(fields, Diagnostic(err)...)...)
	}
	if o.Status == 429 {
		w.Header().Set("Retry-After", "1")
	}
	Send(w, o.Status, registry.Row{"error": o.Payload(r.Header.Get("X-Request-ID"))})
}

// Reply sends an operation's successful result with status.
func Reply[T any](w http.ResponseWriter, status int, v T, e error) error {
	if e == nil {
		Send(w, status, v)
	}
	return e
}

// SendRef sends a mutation result with its revision as the ETag.
func SendRef(w http.ResponseWriter, status int, v op.Ref) {
	w.Header().Set("ETag", strconv.FormatInt(v.Revision, 10))
	Send(w, status, v)
}

// Actor is the console caller of an operation.
func Actor(r *http.Request, p access.Principal) op.Actor {
	return op.ActorFrom(r.Context(), p, r.Header.Get("X-Request-ID"))
}

// ExpectedRevision reads the required If-Match revision.
func ExpectedRevision(r *http.Request) (int64, error) {
	v := strings.Trim(r.Header.Get("If-Match"), `"`)
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 1 {
		return 0, op.RevisionRequired()
	}
	return n, nil
}

// IfMatch returns the If-Match revision, or 0 for the operation to reject.
func IfMatch(r *http.Request) int64 {
	n, _ := ExpectedRevision(r)
	return n
}
