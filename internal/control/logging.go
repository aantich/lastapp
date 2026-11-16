package control

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/aantich/lastapp/internal/registry"
)

type requestLogKey struct{}

type requestLog struct {
	id  string
	log *slog.Logger
}

// requestLogger returns the request-scoped logger (request_id, method) placed
// in ctx by observe, or the default logger outside a request.
func requestLogger(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(requestLogKey{}).(requestLog); ok {
		return v.log
	}
	return slog.Default()
}

func requestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestLogKey{}).(requestLog)
	return v.id
}

// logger is the Server's logger for ctx: request-scoped inside an HTTP
// request, otherwise the Server's base logger (workers).
func (s *Server) logger(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(requestLogKey{}).(requestLog); ok {
		return v.log
	}
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// observe assigns the request ID (the single source of X-Request-ID), puts a
// request-scoped logger into the context and writes one access log line.
// Only the method, matched route pattern, status, size and duration are
// logged: never the query string, body, cookies or credential headers.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := registry.NewID()
		r.Header.Set("X-Request-ID", rid)
		w.Header().Set("X-Request-ID", rid)
		base := s.Log
		if base == nil {
			base = slog.Default()
		}
		log := base.With("request_id", rid, "method", r.Method)
		r = r.WithContext(context.WithValue(r.Context(), requestLogKey{}, requestLog{rid, log}))
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		log.Info("http request", "route", r.Pattern, "status", rec.status, "bytes", rec.bytes, "duration_ms", time.Since(start).Milliseconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, e := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, e
}

// Flush keeps streaming handlers working through the recorder.
func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }
