package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type fakeResumer struct {
	got    []usecase.ResumeEvent
	tenant []string
	err    error
}

func (f *fakeResumer) Handle(ctx context.Context, ev usecase.ResumeEvent) error {
	f.got = append(f.got, ev)
	id, _ := tenant.TenantID(ctx)
	f.tenant = append(f.tenant, id)
	return f.err
}

func statusEvent(id, tenantID string, payload map[string]any) commoneventbus.Event {
	p, _ := json.Marshal(payload)
	return commoneventbus.Event{ID: id, TenantID: tenantID, Payload: p}
}

func TestClarificationResumeConsumer_Handle(t *testing.T) {
	f := &fakeResumer{}
	c := NewClarificationResumeConsumer(nil, f)
	ok := map[string]any{"request_id": "req-1", "to": "analyzing", "trigger": "information_provided"}

	if err := c.Handle(context.Background(), statusEvent("e1", "t1", ok)); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 || f.got[0] != (usecase.ResumeEvent{ID: "e1", TenantID: "t1", RequestID: "req-1", To: "analyzing", Trigger: "information_provided"}) || f.tenant[0] != "t1" {
		t.Fatalf("%+v %v", f.got, f.tenant)
	}
	// irrelevant or unreadable events are acked without reaching the use case
	for name, ev := range map[string]commoneventbus.Event{
		"other trigger": statusEvent("e2", "t1", map[string]any{"request_id": "req-1", "to": "classifying", "trigger": "start_classification"}),
		"no request":    statusEvent("e3", "t1", map[string]any{"to": "analyzing", "trigger": "information_provided"}),
		"no tenant":     statusEvent("e4", "", ok),
		"not json":      {ID: "e5", TenantID: "t1", Payload: []byte("{")},
	} {
		if err := c.Handle(context.Background(), ev); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(f.got) != 1 {
		t.Fatalf("irrelevant events reached the resumer: %d", len(f.got))
	}
	// a failure that redelivery can fix is returned
	f.err = errors.New("transient")
	if err := c.Handle(context.Background(), statusEvent("e6", "t1", ok)); err == nil {
		t.Fatal("a transient error must be returned so the event is redelivered")
	}
}
