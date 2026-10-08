//go:build integration

package eventbus

import (
	"context"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type countingResumer struct {
	mu  sync.Mutex
	ids []string
}

func (c *countingResumer) Handle(_ context.Context, ev usecase.ResumeEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids = append(c.ids, ev.RequestID+"/"+ev.ID+"/"+ev.To)
	return nil
}

func (c *countingResumer) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.ids...)
}

// Real JetStream: the resume durable is independent of the classification durable on the same subject,
// only information_provided events reach it, and a redelivered envelope keeps its id (the dedupe key).
func TestClarificationResumeConsumer_NATS_OnlyInformationProvided(t *testing.T) {
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

	resumer := &countingResumer{}
	classifier := &recordingTrigger{}
	runCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = NewClarificationResumeConsumer(sub, resumer).Run(runCtx) }()
	go func() { defer wg.Done(); _ = NewClassificationConsumer(sub, classifier).Run(runCtx) }()

	mk := func(id, to, trigger string) commoneventbus.Event {
		return statusEvent(id, "11111111-1111-1111-1111-111111111111", map[string]any{"request_id": "req-1", "to": to, "trigger": trigger})
	}
	for _, ev := range []commoneventbus.Event{
		mk("evt-a", "analyzing", "information_provided"),
		mk("evt-a", "analyzing", "information_provided"),
		mk("evt-b", "classifying", "start_classification"),
		mk("evt-c", "awaiting_information", "information_required"),
	} {
		if err := pub.Publish(ctx, domain.SubjectRequestStatusChanged, ev); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for (len(resumer.snapshot()) < 2 || len(classifier.snapshot()) < 1) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	got := resumer.snapshot()
	if len(got) != 2 || got[0] != "req-1/evt-a/analyzing" || got[1] != "req-1/evt-a/analyzing" {
		t.Fatalf("resumer calls = %v, want two deliveries of evt-a and nothing for the other triggers", got)
	}
	if cls := classifier.snapshot(); len(cls) != 1 || cls[0] != "req-1/evt-b" {
		t.Fatalf("the classification consumer must be unaffected: %v", cls)
	}
	stop()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumers did not stop with their context")
	}
}
