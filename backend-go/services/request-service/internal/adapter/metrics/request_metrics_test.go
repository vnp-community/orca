package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func scrape(t *testing.T, s *Set) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	b, _ := io.ReadAll(rec.Result().Body)
	return string(b)
}

func event(t *testing.T, subject string, p map[string]any) domain.OutboxEvent {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return domain.OutboxEvent{Subject: subject, Payload: b}
}

var allSeries = []string{
	"orca_request_created_total", "orca_request_transitions_total", "orca_request_classification_total",
	"orca_request_classification_confidence", "orca_request_approvals_total", "orca_request_approval_wait_seconds",
	"orca_request_approvals_pending", "orca_request_stuck", "orca_request_returned_total",
	"orca_request_ai_generation_seconds", "orca_request_task_outcome_total", "orca_request_outbox_pending",
}

type fakeSamples struct {
	pending map[string]int
	stuck   map[string]int
	outbox  int
	err     error
}

func (f *fakeSamples) PendingApprovalsBySubject(context.Context) (map[string]int, error) {
	return f.pending, f.err
}
func (f *fakeSamples) CountStuck(_ context.Context, status string, _ time.Time) (int, error) {
	return f.stuck[status], f.err
}
func (f *fakeSamples) CountOutboxPending(context.Context) (int, error) { return f.outbox, f.err }

// Every series of CR-REQ-024 section 2.9 must be served; this fails when one is renamed or dropped.
func TestRequestMetrics_ServesAllTwelveSeries(t *testing.T) {
	s := New()
	ctx := context.Background()
	s.OnEvent(ctx, event(t, domain.SubjectRequestCreated, map[string]any{"source_provider": "jira"}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestStatusChanged, map[string]any{"type": "bug", "from": "analyzing", "to": "planning"}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestTypeConfirmed, map[string]any{"type_source": "ai"}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestClassified, map[string]any{"failed": false, "confidence": 0.82}))
	s.OnEvent(ctx, event(t, domain.SubjectApprovalDecided, map[string]any{"subject_type": "plan", "decision": "approved", "wait_seconds": 120.0}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestReturned, map[string]any{"stage": "plan"}))
	s.ObserveAIGeneration("classification", "ok", 2*time.Second)
	s.ObserveTaskOutcome("succeeded")
	s.SampleOnce(ctx, &fakeSamples{pending: map[string]int{"plan": 2}, stuck: map[string]int{"classifying": 1}, outbox: 4},
		StuckThresholds{"classifying": time.Minute}, time.Now(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	out := scrape(t, s)
	for _, name := range allSeries {
		if !strings.Contains(out, "\n"+name) && !strings.HasPrefix(out, name) && !strings.Contains(out, "# TYPE "+name+" ") {
			t.Errorf("series %s is not served", name)
		}
	}
	for _, want := range []string{
		`orca_request_created_total{source_provider="jira"} 1`,
		`orca_request_transitions_total{from="analyzing",to="planning",type="bug"} 1`,
		`orca_request_classification_total{outcome="confirmed_as_proposed"} 1`,
		`orca_request_approvals_total{outcome="approved",subject_type="plan"} 1`,
		`orca_request_approvals_pending{subject_type="plan"} 2`,
		`orca_request_stuck{status="classifying"} 1`,
		`orca_request_returned_total{returned_from_stage="plan"} 1`,
		`orca_request_task_outcome_total{outcome="succeeded"} 1`,
		`orca_request_outbox_pending 4`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing sample %q in:\n%s", want, out)
		}
	}
}

func TestRequestMetrics_ClassificationOutcomes(t *testing.T) {
	s := New()
	ctx := context.Background()
	s.OnEvent(ctx, event(t, domain.SubjectRequestClassified, map[string]any{"failed": true}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestTypeConfirmed, map[string]any{"type_source": "human"}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestTypeChanged, map[string]any{"from": "bug", "to": "task"}))
	s.OnEvent(ctx, event(t, domain.SubjectRequestTypeConfirmed, map[string]any{"type_source": "ai"}))
	for outcome, want := range map[string]float64{"failed": 1, "changed": 2, "confirmed_as_proposed": 1} {
		if got := testutil.ToFloat64(s.classification.WithLabelValues(outcome)); got != want {
			t.Errorf("classification{%s} = %v, want %v", outcome, got, want)
		}
	}
}

// Nothing user-controlled or id-like may become a label value.
func TestRequestMetrics_NoIdentifierEverBecomesALabel(t *testing.T) {
	s := New()
	ctx := context.Background()
	tenantID, requestID, approvalID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, subject := range []string{domain.SubjectRequestCreated, domain.SubjectRequestStatusChanged, domain.SubjectRequestClassified,
		domain.SubjectRequestTypeConfirmed, domain.SubjectRequestTypeChanged, domain.SubjectRequestReturned, domain.SubjectApprovalDecided} {
		ev := event(t, subject, map[string]any{
			"request_id": requestID, "approval_id": approvalID, "tenant_id": tenantID, "title": "TITLE-MARKER", "reason": "REASON-MARKER",
			"actor_id": uuid.NewString(), "source_ref": "ENG-1", "source_provider": "jira", "type": "bug", "from": "new", "to": "classifying",
			"stage": "plan", "subject_type": "plan", "decision": "approved", "wait_seconds": 3.0, "confidence": 0.5,
		})
		ev.TenantID = tenantID
		s.OnEvent(ctx, ev)
	}
	s.ObserveAIGeneration("classification", "ok", time.Second)
	out := scrape(t, s)
	uuidRe := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	if m := uuidRe.FindString(out); m != "" {
		t.Fatalf("a UUID %s reached /metrics", m)
	}
	for _, marker := range []string{"TITLE-MARKER", "REASON-MARKER", "ENG-1", "tenant_id", "request_id", "approval_id"} {
		if strings.Contains(out, marker) {
			t.Fatalf("%q reached /metrics", marker)
		}
	}
}

func TestRequestMetrics_UnreadablePayloadIsIgnored(t *testing.T) {
	s := New()
	s.OnEvent(context.Background(), domain.OutboxEvent{Subject: domain.SubjectRequestCreated, Payload: []byte("{")})
	if got := testutil.CollectAndCount(s.created); got != 0 {
		t.Fatalf("%d series from a broken payload", got)
	}
}

func TestSampler_KeepsLastValueWhenTheDatabaseFails(t *testing.T) {
	s := New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ok := &fakeSamples{pending: map[string]int{"plan": 3}, stuck: map[string]int{"planning": 2}, outbox: 9}
	s.SampleOnce(context.Background(), ok, StuckThresholds{"planning": time.Minute}, time.Now(), log)
	s.SampleOnce(context.Background(), &fakeSamples{err: errors.New("db down")}, StuckThresholds{"planning": time.Minute}, time.Now(), log)
	if got := testutil.ToFloat64(s.outboxPending); got != 9 {
		t.Errorf("outbox gauge %v after a failed sample, want the previous 9", got)
	}
	if got := testutil.ToFloat64(s.approvalsPend.WithLabelValues("plan")); got != 3 {
		t.Errorf("approvals gauge %v, want 3", got)
	}
}

func TestSampler_ZeroesSubjectsThatNoLongerHavePendingApprovals(t *testing.T) {
	s := New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.SampleOnce(context.Background(), &fakeSamples{pending: map[string]int{"plan": 3, "solution": 1}}, nil, time.Now(), log)
	s.SampleOnce(context.Background(), &fakeSamples{pending: map[string]int{"solution": 1}}, nil, time.Now(), log)
	if got := testutil.ToFloat64(s.approvalsPend.WithLabelValues("plan")); got != 0 {
		t.Errorf("plan gauge %v, want 0 once nothing is pending", got)
	}
}

type countingSamples struct {
	mu    sync.Mutex
	calls int
}

func (c *countingSamples) PendingApprovalsBySubject(context.Context) (map[string]int, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return nil, nil
}
func (c *countingSamples) CountStuck(context.Context, string, time.Time) (int, error) { return 0, nil }
func (c *countingSamples) CountOutboxPending(context.Context) (int, error)            { return 0, nil }

func TestSampler_SamplesImmediatelyThenPeriodicallyAndStopsOnCancel(t *testing.T) {
	s := New()
	src := &countingSamples{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RunSampler(ctx, src, nil, 10*time.Millisecond, nil)
		close(done)
	}()
	deadline := time.After(5 * time.Second)
	for {
		src.mu.Lock()
		n := src.calls
		src.mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d samples", n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("sampler did not stop after cancel")
	}
}

// The alert rules (deploy/alerts/request.rules.yaml) only help if every series they query is really served.
func TestAlertRulesQueryOnlySeriesThatExist(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "deploy", "alerts", "request.rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := New()
	s.SampleOnce(context.Background(), &fakeSamples{}, StuckThresholds{"classifying": time.Minute}, time.Now(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	known := map[string]bool{}
	for _, name := range allSeries {
		known[name] = true
	}
	re := regexp.MustCompile(`orca_request_[a-z_]+`)
	seen := 0
	for _, name := range re.FindAllString(string(raw), -1) {
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, "_bucket"), "_sum"), "_count")
		seen++
		if !known[base] {
			t.Errorf("alert rule queries %q, which request-service does not serve", name)
		}
	}
	if seen == 0 {
		t.Fatal("no orca_request_ series found in request.rules.yaml: wrong path?")
	}
}
