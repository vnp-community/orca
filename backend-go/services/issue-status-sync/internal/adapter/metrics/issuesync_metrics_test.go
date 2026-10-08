package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func scrape(t *testing.T, s *Set) string {
	t.Helper()
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	return rr.Body.String()
}

func TestMetricsExposeTheThreeIssueSyncSeries(t *testing.T) {
	s := New()
	s.ObserveRequestEvent("status_changed", "applied")
	s.ObserveRequestOwnedSkip("worktree")
	s.ObserveJiraTransition(300 * time.Millisecond)
	out := scrape(t, s)
	for _, want := range []string{
		`orca_issuesync_request_events_total{event="status_changed",result="applied"} 1`,
		`orca_issuesync_skipped_request_owned_total{source="worktree"} 1`,
		`orca_issuesync_jira_transition_seconds_count 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

func TestMetricsNeverContainIDs(t *testing.T) {
	// Callers only pass fixed enum values; this guards the metric's own label set.
	s := New()
	id := uuid.NewString()
	s.ObserveRequestEvent("status_changed", "failed")
	out := scrape(t, s)
	if strings.Contains(out, id) || strings.Contains(out, "tenant_id") || strings.Contains(out, "request_id") {
		t.Error("an id-like label leaked into /metrics")
	}
}
