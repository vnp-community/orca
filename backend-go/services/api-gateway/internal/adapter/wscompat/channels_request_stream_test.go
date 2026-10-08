package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

var streamNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func requestEvent(tenantID string, at time.Time, payload string) commoneventbus.Event {
	return commoneventbus.Event{ID: "e", TenantID: tenantID, OccurredAt: at, Payload: json.RawMessage(payload)}
}

// openRequestStream subscribes and collects the frames delivered within a short window.
func openRequestStream(t *testing.T, bus ephemeralSubscriber, fake *fakeRequestClient, id Identity, args string, events []requestEventMapping) ([]RequestEventFrame, error) {
	t.Helper()
	r := NewRegistry()
	registerRequestStream(r, bus, fake, events, func() time.Time { return streamNow })
	sh, ok := r.StreamHandlerFor("request.subscribe")
	if !ok {
		t.Fatal("request.subscribe is not registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := sh(ctx, id, []json.RawMessage{json.RawMessage(args)})
	if err != nil {
		return nil, err
	}
	var frames []RequestEventFrame
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case ev, open := <-ch:
			if !open {
				return frames, nil
			}
			if ev.Channel != "request.event" || len(ev.Args) != 1 {
				t.Fatalf("push = %+v", ev)
			}
			frames = append(frames, ev.Args[0].(RequestEventFrame))
		case <-timeout:
			return frames, nil
		}
	}
}

func TestRequestStream_FourteenEventsMapToTheirType(t *testing.T) {
	if len(requestEventRegistry) != 14 {
		t.Fatalf("registry has %d rows, want the 14 of CONTRACT section 3", len(requestEventRegistry))
	}
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{}}
	for _, m := range requestEventRegistry {
		bus.events[m.Subject] = []commoneventbus.Event{requestEvent("t1", streamNow,
			`{"request_id":"r1","to":"analyzing","type":"bug","trigger":"reopen","approval_id":"a1","subject_type":"solution","solution_id":"s1","phase_task_id":"ph","plan_task_id":"pl","title":"SECRET-TITLE","body":"SECRET-BODY","reporter_id":"u1"}`)}
	}
	frames, err := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", UserID: "u1"}, `{}`, requestEventRegistry)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RequestEventFrame{}
	for _, f := range frames {
		got[f.EventType] = f
	}
	for _, m := range requestEventRegistry {
		f, ok := got[m.EventType]
		if !ok {
			t.Errorf("no frame for %s (%s)", m.EventType, m.Subject)
			continue
		}
		if f.RequestID != "r1" || f.Status != "analyzing" || f.Type != "bug" || f.OccurredAt != "2026-10-08T12:00:00Z" || f.ApprovalID != "a1" ||
			f.SolutionID != "s1" || f.PhaseTaskID != "ph" || f.PlanTaskID != "pl" || f.Trigger != "reopen" || f.SubjectType != "solution" {
			t.Errorf("%s: frame = %+v", m.EventType, f)
		}
		b, _ := json.Marshal(f)
		var keys map[string]any
		_ = json.Unmarshal(b, &keys)
		for k := range keys {
			if strings.Contains(k, "_") || k == "title" || k == "body" {
				t.Errorf("%s: frame key %q is not whitelisted camelCase", m.EventType, k)
			}
		}
		if strings.Contains(string(b), "SECRET") {
			t.Errorf("%s: frame leaks content: %s", m.EventType, b)
		}
	}
	if len(frames) != 14 {
		t.Errorf("got %d frames, want 14", len(frames))
	}
}

func TestRequestStream_FiltersTenantReplayAndID(t *testing.T) {
	const subject = "orca.request.request.status_changed"
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{subject: {
		requestEvent("t1", streamNow, `{"request_id":"r1","to":"planning","reporter_id":"u1"}`),
		requestEvent("t2", streamNow, `{"request_id":"r1","to":"planning","reporter_id":"u1"}`),               // other tenant
		requestEvent("t1", streamNow.Add(-time.Hour), `{"request_id":"r1","to":"old","reporter_id":"u1"}`),    // replayed history
		requestEvent("t1", streamNow.Add(-time.Second), `{"request_id":"r1","to":"edge","reporter_id":"u1"}`), // inside the grace
		requestEvent("t1", streamNow, `{"request_id":"r2","to":"other-request","reporter_id":"u1"}`),          // other request
		requestEvent("t1", streamNow, `not json`),                                                             // malformed
		requestEvent("t1", streamNow, `{"to":"planning"}`),                                                    // no request id
	}}}
	rows := []requestEventMapping{{Stream: "REQUEST", Subject: subject, EventType: "request.status_changed"}}
	id := Identity{TenantID: "t1", UserID: "u1"}

	frames, err := openRequestStream(t, bus, &fakeRequestClient{}, id, `{"id":"r1"}`, rows)
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, f := range frames {
		statuses = append(statuses, f.Status)
	}
	if strings.Join(statuses, ",") != "planning,edge" {
		t.Errorf("with id: statuses = %v, want planning,edge (no other tenant, no replay, no other request)", statuses)
	}

	frames, err = openRequestStream(t, bus, &fakeRequestClient{}, id, `{}`, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 { // planning, edge, other-request: same tenant, fresh, reported by u1
		t.Errorf("tenant-wide: %d frames, want 3", len(frames))
	}
}

func TestRequestStream_TenantWideVisibility(t *testing.T) {
	const subject = "orca.request.request.created"
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{subject: {
		requestEvent("t1", streamNow, `{"request_id":"mine","reporter_id":"u1"}`),
		requestEvent("t1", streamNow, `{"request_id":"caused","reporter_id":"u9","actor_id":"u1"}`),
		requestEvent("t1", streamNow, `{"request_id":"theirs","reporter_id":"u9","actor_id":"u9"}`),
	}}}
	rows := []requestEventMapping{{Stream: "REQUEST", Subject: subject, EventType: "request.created"}}
	ids := func(frames []RequestEventFrame) string {
		var out []string
		for _, f := range frames {
			out = append(out, f.RequestID)
		}
		return strings.Join(out, ",")
	}
	member, _ := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", UserID: "u1", Role: "user"}, `{}`, rows)
	if got := ids(member); got != "mine,caused" {
		t.Errorf("member sees %q, want mine,caused", got)
	}
	admin, _ := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", UserID: "boss", Role: "admin"}, `{}`, rows)
	if len(admin) != 3 {
		t.Errorf("admin sees %d frames, want 3", len(admin))
	}
	anon, _ := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", Role: "user"}, `{}`, rows)
	if len(anon) != 0 {
		t.Errorf("a session without a user must see nothing tenant-wide, saw %d", len(anon))
	}
}

type countingBus struct{ calls atomic.Int32 }

func (b *countingBus) SubscribeEphemeral(ctx context.Context, _, _ string, _ commoneventbus.Handler) error {
	b.calls.Add(1)
	<-ctx.Done()
	return nil
}

func TestRequestStream_PermissionProbeFailureOpensNoSubscription(t *testing.T) {
	bus := &countingBus{}
	fake := &fakeRequestClient{err: status.Error(codes.NotFound, "REQUEST_NOT_FOUND: no such request")}
	_, err := openRequestStream(t, bus, fake, Identity{TenantID: "t1", UserID: "u1"}, `{"id":"r1"}`, requestEventRegistry)
	if err == nil || err.Error() != "REQUEST_NOT_FOUND: no such request" {
		t.Fatalf("err = %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if bus.calls.Load() != 0 {
		t.Errorf("SubscribeEphemeral called %d times after a failed GetRequest", bus.calls.Load())
	}
	call := fake.last()
	if call.rpc != "GetRequest" || len(call.md.Get("x-orca-tenant-id")) == 0 || call.md.Get("x-orca-tenant-id")[0] != "t1" {
		t.Errorf("probe rpc=%q md=%v: identity must be attached on the stream path", call.rpc, call.md)
	}
}

func TestRequestStream_NoProbeWithoutID(t *testing.T) {
	fake := &fakeRequestClient{}
	if _, err := openRequestStream(t, &fakeEphemeralSubscriber{}, fake, Identity{TenantID: "t1", UserID: "u1"}, `{}`, requestEventRegistry); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 0 {
		t.Errorf("a tenant-wide subscription made %d RPCs", fake.count())
	}
}

func TestRequestStream_NewRegistryRowNeedsNoLogicChange(t *testing.T) {
	rows := append([]requestEventMapping{}, requestEventRegistry...)
	rows = append(rows, requestEventMapping{Stream: "REQUEST", Subject: "orca.request.clarification.requested", EventType: "clarification.requested"})
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{
		"orca.request.clarification.requested": {requestEvent("t1", streamNow, `{"request_id":"r1","reporter_id":"u1"}`)},
	}}
	frames, err := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", UserID: "u1"}, `{}`, rows)
	if err != nil || len(frames) != 1 || frames[0].EventType != "clarification.requested" {
		t.Fatalf("frames=%+v err=%v", frames, err)
	}
}

func TestRequestStream_CustomExtractOverridesTheDefault(t *testing.T) {
	rows := []requestEventMapping{{Stream: "REQUEST", Subject: "s.x", EventType: "x.y",
		Extract: func(json.RawMessage) (requestEventFields, bool) {
			return requestEventFields{RequestID: "from-extract", ReporterID: "u1"}, true
		}}}
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{"s.x": {requestEvent("t1", streamNow, `{}`)}}}
	frames, _ := openRequestStream(t, bus, &fakeRequestClient{}, Identity{TenantID: "t1", UserID: "u1"}, `{}`, rows)
	if len(frames) != 1 || frames[0].RequestID != "from-extract" {
		t.Fatalf("frames = %+v", frames)
	}
}

func TestRequestStream_UnavailableWithoutClientOrBus(t *testing.T) {
	r := NewRegistry()
	registerRequestStream(r, nil, &fakeRequestClient{}, requestEventRegistry, time.Now)
	sh, _ := r.StreamHandlerFor("request.subscribe")
	if _, err := sh(context.Background(), Identity{TenantID: "t1"}, nil); err == nil || !strings.HasPrefix(err.Error(), "REQUEST_UNAVAILABLE") {
		t.Errorf("nil bus: %v", err)
	}
	r = NewRegistry()
	registerRequestStream(r, &fakeEphemeralSubscriber{}, nil, requestEventRegistry, time.Now)
	sh, _ = r.StreamHandlerFor("request.subscribe")
	if _, err := sh(context.Background(), Identity{TenantID: "t1"}, []json.RawMessage{json.RawMessage(`{"id":"r"}`)}); err == nil || !strings.HasPrefix(err.Error(), "REQUEST_UNAVAILABLE") {
		t.Errorf("nil client with id: %v", err)
	}
}
