//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type recordingTrigger struct {
	mu    sync.Mutex
	calls []string // request_id/event_id
}

func (r *recordingTrigger) Run(_ context.Context, requestID, eventID, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, requestID+"/"+eventID)
	return nil
}

func (r *recordingTrigger) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// Real JetStream: a redelivered envelope (same id) reaches the trigger with the same event id,
// which is what lets the use case dedupe, and events for other targets never reach it.
func TestClassificationConsumer_NATS_RedeliveryKeepsEventID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "nats:2.10-alpine", Cmd: []string{"-js"}, ExposedPorts: []string{"4222/tcp"},
			WaitingFor: wait.ForLog("Server is ready"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start nats: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(c) }()
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "4222")
	pub, sub, closeBus, err := commoneventbus.Connect(ctx, "nats://"+host+":"+port.Port())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "REQUEST", []string{"orca.request.>"}); err != nil {
		t.Fatal(err)
	}

	trigger := &recordingTrigger{}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = NewClassificationConsumer(sub, trigger).Run(runCtx) }()

	mk := func(id, to string) commoneventbus.Event {
		p, _ := json.Marshal(map[string]any{"request_id": "req-1", "to": to, "trigger": "start_classification"})
		return commoneventbus.Event{ID: id, TenantID: "11111111-1111-1111-1111-111111111111", OccurredAt: time.Now().UTC(), Version: 1, Payload: p}
	}
	for _, ev := range []commoneventbus.Event{mk("evt-a", "classifying"), mk("evt-a", "classifying"), mk("evt-b", "analyzing")} {
		if err := pub.Publish(ctx, domain.SubjectRequestStatusChanged, ev); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for len(trigger.snapshot()) < 2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // give a wrongly delivered third call time to show up
	got := trigger.snapshot()
	if len(got) != 2 || got[0] != "req-1/evt-a" || got[1] != "req-1/evt-a" {
		t.Fatalf("trigger calls = %v, want two deliveries of req-1/evt-a and nothing for the analyzing event", got)
	}
	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop with its context")
	}
}
