package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type fakeEnqueuer struct {
	got    []usecase.PlanAutoEvent
	tenant []string
	err    error
}

func (f *fakeEnqueuer) Handle(ctx context.Context, ev usecase.PlanAutoEvent) error {
	f.got = append(f.got, ev)
	id, _ := tenant.TenantID(ctx)
	f.tenant = append(f.tenant, id)
	return f.err
}

func TestPlanAutoConsumer_Handle(t *testing.T) {
	f := &fakeEnqueuer{}
	c := NewPlanAutoConsumer(nil, f)
	ok := map[string]any{"request_id": "req-1", "project_id": "p-1", "to": "planning", "trigger": "analysis_approved"}

	if err := c.Handle(context.Background(), statusEvent("e1", "t1", ok)); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 || f.got[0] != (usecase.PlanAutoEvent{ID: "e1", TenantID: "t1", RequestID: "req-1", ProjectID: "p-1", To: "planning", Trigger: "analysis_approved"}) || f.tenant[0] != "t1" {
		t.Fatalf("%+v %v", f.got, f.tenant)
	}
	confirmed := map[string]any{"request_id": "req-2", "to": "planning", "trigger": "type_confirmed"}
	if err := c.Handle(context.Background(), statusEvent("e2", "t1", confirmed)); err != nil || len(f.got) != 2 {
		t.Fatalf("type_confirmed: %v %d", err, len(f.got))
	}

	for name, ev := range map[string]commoneventbus.Event{
		"classification": statusEvent("e3", "t1", map[string]any{"request_id": "req-1", "to": "classifying", "trigger": "start_classification"}),
		"resume":         statusEvent("e4", "t1", map[string]any{"request_id": "req-1", "to": "analyzing", "trigger": "information_provided"}),
		"replan":         statusEvent("e5", "t1", map[string]any{"request_id": "req-1", "to": "planning", "trigger": "plan_revision"}),
		"other status":   statusEvent("e6", "t1", map[string]any{"request_id": "req-1", "to": "executing", "trigger": "analysis_approved"}),
		"no request":     statusEvent("e7", "t1", map[string]any{"to": "planning", "trigger": "analysis_approved"}),
		"no tenant":      statusEvent("e8", "", ok),
		"not json":       {ID: "e9", TenantID: "t1", Payload: []byte("{")},
	} {
		if err := c.Handle(context.Background(), ev); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(f.got) != 2 {
		t.Fatalf("irrelevant events reached the use case: %d", len(f.got))
	}
	f.err = errors.New("transient")
	if err := c.Handle(context.Background(), statusEvent("e10", "t1", ok)); err == nil {
		t.Fatal("a failure must be returned so the event is redelivered")
	}
}

func TestPlanAutoConsumer_IgnoresEventsOlderThanTheStartWindow(t *testing.T) {
	f := &fakeEnqueuer{}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	c := NewPlanAutoConsumer(nil, f).WithMaxEventAge(30 * time.Minute)
	c.now = func() time.Time { return now }
	payload := map[string]any{"request_id": "req-1", "to": "planning", "trigger": "analysis_approved"}

	old := statusEvent("old", "t1", payload)
	old.OccurredAt = now.Add(-5 * 24 * time.Hour)
	if err := c.Handle(context.Background(), old); err != nil || len(f.got) != 0 {
		t.Fatalf("a replayed event of days ago must be acked and ignored: err=%v jobs=%d", err, len(f.got))
	}
	fresh := statusEvent("fresh", "t1", payload)
	fresh.OccurredAt = now.Add(-2 * time.Minute)
	if err := c.Handle(context.Background(), fresh); err != nil || len(f.got) != 1 {
		t.Fatalf("a recent event must be enqueued: err=%v jobs=%d", err, len(f.got))
	}
	unstamped := statusEvent("unstamped", "t1", payload)
	if err := c.Handle(context.Background(), unstamped); err != nil || len(f.got) != 2 {
		t.Fatalf("an event without a timestamp is kept: err=%v jobs=%d", err, len(f.got))
	}
}
