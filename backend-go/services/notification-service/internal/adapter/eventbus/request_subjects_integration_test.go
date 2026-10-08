//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/notification-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// Durable request events published while notification-service is down reach the
// recipients named in user_ids after it starts, and a redelivered id is dropped.
func TestRequestSubjects_DurableEventsPublishedWhileConsumerDownAreDelivered(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pub, cons, closeBus, err := commoneventbus.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"}); err != nil {
		t.Fatal(err)
	}

	mk := func(id string, userIDs []string) commoneventbus.Event {
		payload, _ := json.Marshal(map[string]any{"request_id": "req-1", "user_ids": userIDs, "title": "t", "body": "b", "deep_link": "/?section=requests&request=req-1"})
		return commoneventbus.Event{ID: id, TenantID: "t1", OccurredAt: time.Now().UTC(), Version: 1, Payload: payload}
	}
	approval := mk("0b6d7d5e-1f1c-5d0a-8a3b-aaaaaaaaaaa1", []string{"u1"})
	clarification := mk("0b6d7d5e-1f1c-5d0a-8a3b-aaaaaaaaaaa2", []string{"u1"})
	expired := mk("0b6d7d5e-1f1c-5d0a-8a3b-aaaaaaaaaaa3", []string{"u1"})
	other := mk("0b6d7d5e-1f1c-5d0a-8a3b-aaaaaaaaaaa4", []string{"someone-else"})
	for subject, ev := range map[string]commoneventbus.Event{
		"orca.request.approval.requested":      approval,
		"orca.request.clarification.requested": clarification,
		"orca.request.clarification.expired":   expired,
	} {
		if err := pub.PublishDedup(ctx, subject, ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := pub.PublishDedup(ctx, "orca.request.approval.requested", other); err != nil {
		t.Fatal(err)
	}

	bc := broadcaster.New()
	events, unsubscribe := bc.Subscribe(ctx, "t1", "u1")
	defer unsubscribe()
	uc := usecase.NewHandleIncomingEvent(bc, &memoryProcessedEvents{seen: map[string]bool{}}, nil, nil, nil)
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go New(cons, uc).Run(runCtx, testLogger())

	got := map[string]string{}
	for len(got) < 3 {
		select {
		case n := <-events:
			got[n.SourceEventID] = n.Type
		case <-time.After(20 * time.Second):
			t.Fatalf("only received %v", got)
		}
	}
	for id, typ := range map[string]string{approval.ID: "request.approval_requested", clarification.ID: "request.clarification_requested", expired.ID: "request.clarification_expired"} {
		if got[id] != typ {
			t.Errorf("event %s: type %q, want %q", id, got[id], typ)
		}
	}
	select {
	case extra := <-events:
		t.Fatalf("unexpected extra notification %+v", extra)
	case <-time.After(1500 * time.Millisecond):
	}
}
