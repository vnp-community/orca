package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/stablyai/orca-go/common/tenant"
)

var recordTime = time.Now()

func newTestHandler(buf *bytes.Buffer) *correlatingHandler {
	return &correlatingHandler{
		inner: slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}
}

func decodeLastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decoding log JSON: %v (raw: %s)", err, buf.String())
	}
	return out
}

// TestCorrelatingHandler_AddsTraceIDWhenSpanValid proves Handle now fulfills
// the doc comment's long-standing promise ("once tracing is wired via
// common/tracing, trace_id") — TASK-BE-FFT-003.
func TestCorrelatingHandler_AddsTraceIDWhenSpanValid(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	rec := slog.NewRecord(recordTime, slog.LevelInfo, "hello", 0)
	if err := h.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeLastLine(t, &buf)
	if out["trace_id"] != sc.TraceID().String() {
		t.Errorf("want trace_id=%s, got %v", sc.TraceID().String(), out["trace_id"])
	}
	if out["span_id"] != sc.SpanID().String() {
		t.Errorf("want span_id=%s, got %v", sc.SpanID().String(), out["span_id"])
	}
}

// TestCorrelatingHandler_NoSpanContext_NoTraceIDField_NoPanic is the
// mandatory regression: outside any span, trace_id/span_id must be absent
// and Handle must not panic.
func TestCorrelatingHandler_NoSpanContext_NoTraceIDField_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	rec := slog.NewRecord(recordTime, slog.LevelInfo, "hello", 0)
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeLastLine(t, &buf)
	if _, ok := out["trace_id"]; ok {
		t.Errorf("expected no trace_id field outside a span, got %v", out["trace_id"])
	}
	if _, ok := out["span_id"]; ok {
		t.Errorf("expected no span_id field outside a span, got %v", out["span_id"])
	}
}

// TestCorrelatingHandler_TenantUserFieldsUnaffected confirms tenant_id/
// user_id keep working exactly as before, alongside the new trace fields.
func TestCorrelatingHandler_TenantUserFieldsUnaffected(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	ctx := tenant.WithTenantID(context.Background(), "t1")
	ctx = tenant.WithUserID(ctx, "u1")

	rec := slog.NewRecord(recordTime, slog.LevelInfo, "hello", 0)
	if err := h.Handle(ctx, rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeLastLine(t, &buf)
	if out["tenant_id"] != "t1" {
		t.Errorf("want tenant_id=t1, got %v", out["tenant_id"])
	}
	if out["user_id"] != "u1" {
		t.Errorf("want user_id=u1, got %v", out["user_id"])
	}
	if _, ok := out["trace_id"]; ok {
		t.Errorf("expected no trace_id field with no span in context, got %v", out["trace_id"])
	}
}

// TestCorrelatingHandler_WithAttrsWithGroup_StillDelegateCorrectly confirms
// the untouched methods still just delegate to inner, wrapped.
func TestCorrelatingHandler_WithAttrsWithGroup_StillDelegateCorrectly(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := withAttrs.(*correlatingHandler); !ok {
		t.Fatalf("WithAttrs should still return a *correlatingHandler, got %T", withAttrs)
	}

	withGroup := h.WithGroup("g")
	if _, ok := withGroup.(*correlatingHandler); !ok {
		t.Fatalf("WithGroup should still return a *correlatingHandler, got %T", withGroup)
	}

	rec := slog.NewRecord(recordTime, slog.LevelInfo, "hello", 0)
	if err := withAttrs.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle after WithAttrs: %v", err)
	}
	out := decodeLastLine(t, &buf)
	if out["k"] != "v" {
		t.Errorf("want k=v from WithAttrs, got %v", out["k"])
	}
}
