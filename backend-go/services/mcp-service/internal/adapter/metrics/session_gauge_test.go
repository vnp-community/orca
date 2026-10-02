package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCounter struct {
	n   int64
	err error
}

func (f *fakeCounter) CountOpenSessions(context.Context) (int64, error) { return f.n, f.err }

func scrape(s *Set) string {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestSessionGaugeSampledAndErrorsCounted(t *testing.T) {
	fc := &fakeCounter{n: 7}
	s := New(fc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Refresh(context.Background())
	if body := scrape(s); !strings.Contains(body, "orca_mcp_sessions_active 7") {
		t.Fatalf("gauge not exposed:\n%s", body)
	}
	fc.err = errors.New("db down")
	s.Refresh(context.Background())
	body := scrape(s)
	if !strings.Contains(body, "orca_mcp_sessions_active 7") || !strings.Contains(body, "orca_mcp_sessions_count_errors_total 1") {
		t.Fatalf("a failed sample must keep the last value and count the error:\n%s", body)
	}
	for _, bad := range []string{`tenant=`, `tenant_id=`, `user=`, `user_id=`} {
		if strings.Contains(body, bad) {
			t.Fatalf("label %q must not appear", bad)
		}
	}
}
