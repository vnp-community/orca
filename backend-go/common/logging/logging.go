// Package logging sets up the structured slog.Logger every service uses,
// with trace/tenant correlation — see
// specs/backend-go/architecture/09-observability-reliability.md
// ("structured logging... every log line carries trace_id, tenant_id,
// service, version") and standards/go-coding-standards.md's rule against
// fmt.Println-style logging.
package logging

import (
	"context"
	"log/slog"
	"os"

	"github.com/stablyai/orca-go/common/tenant"
)

// New returns a JSON slog.Logger tagged with the service name and version,
// suitable as the process-wide logger constructed once in cmd/server/main.go
// and passed down explicitly (never a package-level global — see
// standards/go-coding-standards.md's "no global mutable state" rule).
func New(serviceName, version string) *slog.Logger {
	handler := &correlatingHandler{
		inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}
	return slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("version", version),
	)
}

// correlatingHandler injects tenant_id/user_id (and, once tracing is wired
// via common/tracing, trace_id) from context into every log record, so a
// handler doesn't have to remember to attach them at every call site.
type correlatingHandler struct {
	inner slog.Handler
}

func (h *correlatingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *correlatingHandler) Handle(ctx context.Context, record slog.Record) error {
	if tid, ok := tenant.TenantID(ctx); ok {
		record.AddAttrs(slog.String("tenant_id", tid))
	}
	if uid, ok := tenant.UserID(ctx); ok {
		record.AddAttrs(slog.String("user_id", uid))
	}
	return h.inner.Handle(ctx, record)
}

func (h *correlatingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &correlatingHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *correlatingHandler) WithGroup(name string) slog.Handler {
	return &correlatingHandler{inner: h.inner.WithGroup(name)}
}
