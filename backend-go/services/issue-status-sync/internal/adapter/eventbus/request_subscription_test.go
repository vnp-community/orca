package eventbus

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/stablyai/orca-go/services/issue-status-sync/internal/usecase"
)

type recordingCommenter struct{ bodies []string }

func (r *recordingCommenter) AddComment(_ context.Context, _, _, _, _, _, body string) error {
	r.bodies = append(r.bodies, body)
	return nil
}

func newRequestSubscriber(opts ...usecase.Option) (*Subscriber, *recordingTracker) {
	tracker := &recordingTracker{}
	return New(nil, usecase.NewSyncIssueStatus(tracker, noScm{}, syncOn{}, memSeen{}, nil, opts...), nil), tracker
}

const executingPayload = `{"request_id":"r1","project_id":"p1","from":"planning","to":"executing","trigger":"approval","type":"bug",` +
	`"actor_id":"user-7","actor_kind":"user","source_provider":"jira","source_site":"https://a.atlassian.net","source_ref":"ENG-5",` +
	`"reporter_id":"user-9","number":12,"version":3,"stage":"x","reason":"y","at":"2026-10-07T00:00:00Z"}`

func TestRequestStatusChangedPayloadDrivesAJiraTransition(t *testing.T) {
	sub, tracker := newRequestSubscriber()
	if err := sub.handleRequestEvent(false)(context.Background(), event(executingPayload)); err != nil {
		t.Fatal(err)
	}
	if tracker.transitions != 1 || tracker.ref != "ENG-5" || tracker.state != "In Progress" || tracker.user != "user-7" || tracker.site != "https://a.atlassian.net" {
		t.Errorf("unexpected transition %+v", tracker)
	}
}

func TestRequestCompletedPayloadMovesIssueToDone(t *testing.T) {
	sub, tracker := newRequestSubscriber()
	tracker.category = "in_progress"
	payload := strings.Replace(executingPayload, `"to":"executing"`, `"to":"completed"`, 1)
	if err := sub.handleRequestEvent(true)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	if tracker.transitions != 1 || tracker.state != "Done" {
		t.Errorf("want Done, got %+v", tracker)
	}
}

func TestRequestMalformedPayloadIsAcknowledged(t *testing.T) {
	sub, tracker := newRequestSubscriber()
	if err := sub.handleRequestEvent(false)(context.Background(), event(`{not json`)); err != nil {
		t.Fatalf("malformed payload must not be redelivered: %v", err)
	}
	if tracker.transitions != 0 {
		t.Error("nothing may be sent")
	}
}

func TestUnknownPayloadFieldsAreIgnored(t *testing.T) {
	sub, tracker := newRequestSubscriber()
	payload := strings.Replace(executingPayload, `"stage":"x"`, `"stage":"x","brand_new_field":{"a":1}`, 1)
	if err := sub.handleRequestEvent(false)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	if tracker.transitions != 1 {
		t.Error("an unknown field must not break the consumer")
	}
}

func TestJiraCommentNeverCarriesPayloadBodyOrTitle(t *testing.T) {
	c := &recordingCommenter{}
	sub, _ := newRequestSubscriber(usecase.WithIssueComments(c, ""))
	payload := strings.Replace(executingPayload, `"stage":"x"`, `"title":"SECRET-TITLE","body":"SECRET-BODY","stage":"x"`, 1)
	if err := sub.handleRequestEvent(false)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	if len(c.bodies) != 1 {
		t.Fatalf("comments = %v", c.bodies)
	}
	if strings.Contains(c.bodies[0], "SECRET") {
		t.Errorf("comment leaked content: %q", c.bodies[0])
	}
}

func TestRequestEventSpanLinksToProducerTrace(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	sub, _ := newRequestSubscriber()
	const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	payload := strings.Replace(executingPayload, `"stage":"x"`, `"traceparent":"`+traceparent+`","stage":"x"`, 1)
	if err := sub.handleRequestEvent(false)(context.Background(), event(payload)); err != nil {
		t.Fatal(err)
	}
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	links := spans[0].Links()
	if len(links) != 1 || links[0].SpanContext.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("links = %+v", links)
	}
	if spans[0].Parent().IsValid() {
		t.Error("the span must be linked, not parented, to the producer trace")
	}
}

func TestRequestEventWithoutTraceparentStillGetsASpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	sub, _ := newRequestSubscriber()
	if err := sub.handleRequestEvent(false)(context.Background(), event(executingPayload)); err != nil {
		t.Fatal(err)
	}
	spans := rec.Ended()
	if len(spans) != 1 || len(spans[0].Links()) != 0 {
		t.Errorf("want one span with no links, got %d", len(spans))
	}
}
