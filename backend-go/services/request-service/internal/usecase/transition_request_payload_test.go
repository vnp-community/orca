package usecase

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// issue-status-sync reads exactly these keys (adapter/eventbus/subscriber.go requestStatusPayload); renaming one breaks Jira sync.
var syncConsumerKeys = []string{"request_id", "project_id", "from", "to", "trigger", "type", "actor_id", "actor_kind",
	"source_provider", "source_site", "source_ref", "reporter_id", "number", "version"}

func rawPayload(t *testing.T, ev domain.OutboxEvent) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(ev.Payload, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestTransitionPayloads_CarryTheFieldsIssueSyncNeeds(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) {
		r.Status = domain.RequestStatusExecuting
		r.Type = domain.RequestTypeTask
		r.SourceProvider, r.SourceSite, r.SourceRef = domain.SourceProviderJira, "https://a.atlassian.net", "ENG-7"
		r.Title = "TITLE-MARKER"
		r.Body = "BODY-MARKER"
	})
	in := TransitionInput{RequestID: r.ID, Trigger: domain.TriggerExecutionFinished, ActorID: "u-actor", ActorKind: domain.ActorKindUser}
	if _, err := newLcTransition(s).Execute(lcCtx(), in); err != nil {
		t.Fatal(err)
	}
	if got := s.subjects(); len(got) != 2 {
		t.Fatalf("events %v", got)
	}
	for _, ev := range s.events {
		m := rawPayload(t, ev)
		for _, k := range syncConsumerKeys {
			if k == "from" || k == "to" || k == "trigger" {
				if ev.Subject == domain.SubjectRequestCompleted {
					continue // completed has no from/to/trigger; the subject itself says what happened
				}
			}
			if _, ok := m[k]; !ok {
				t.Errorf("%s payload lacks %q: %s", ev.Subject, k, ev.Payload)
			}
		}
		if m["source_provider"] != "jira" || m["source_site"] != "https://a.atlassian.net" || m["source_ref"] != "ENG-7" || m["reporter_id"] != r.ReporterID {
			t.Errorf("%s: source/reporter wrong: %s", ev.Subject, ev.Payload)
		}
		if m["number"].(float64) != float64(r.Number) || m["version"].(float64) < 2 {
			t.Errorf("%s: number/version wrong: %s", ev.Subject, ev.Payload)
		}
		if m["actor_id"] != "u-actor" || m["actor_kind"] != "user" {
			t.Errorf("%s: actor wrong: %s", ev.Subject, ev.Payload)
		}
		if strings.Contains(string(ev.Payload), "TITLE-MARKER") || strings.Contains(string(ev.Payload), "BODY-MARKER") {
			t.Errorf("%s leaks title or body: %s", ev.Subject, ev.Payload)
		}
	}
}

func TestTransitionPayloads_TraceparentOnlyWithAValidSpan(t *testing.T) {
	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	if _, err := newLcTransition(s).Execute(lcCtx(), TransitionInput{RequestID: r.ID, Trigger: domain.TriggerExecutionFinished}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range s.events {
		if _, ok := rawPayload(t, ev)["traceparent"]; ok {
			t.Errorf("%s: no active span, traceparent must be omitted: %s", ev.Subject, ev.Payload)
		}
	}
}

func TestTransitionPayloads_TraceparentLinksToTheTransitionSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)).Tracer("test")

	s := newLcStore()
	r := s.seed(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting; r.Type = domain.RequestTypeTask })
	ctx, root := tracer.Start(lcCtx(), "rpc")
	uc := newLcTransition(s)
	uc.tracer = tracer
	if _, err := uc.Execute(ctx, TransitionInput{RequestID: r.ID, Trigger: domain.TriggerExecutionFinished}); err != nil {
		t.Fatal(err)
	}
	root.End()

	var transition sdktrace.ReadOnlySpan
	for _, sp := range rec.Ended() {
		if sp.Name() == "request.Transition" {
			transition = sp
		}
	}
	if transition == nil {
		t.Fatal("no request.Transition span recorded")
	}
	if transition.Parent().SpanID() != root.SpanContext().SpanID() {
		t.Error("the transition span must be a child of the RPC span")
	}
	want := "00-" + transition.SpanContext().TraceID().String() + "-" + transition.SpanContext().SpanID().String() + "-01"
	for _, ev := range s.events {
		if got := rawPayload(t, ev)["traceparent"]; got != want {
			t.Errorf("%s traceparent = %v, want %s", ev.Subject, got, want)
		}
	}
}
