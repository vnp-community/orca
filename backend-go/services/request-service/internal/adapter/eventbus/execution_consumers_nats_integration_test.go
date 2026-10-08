//go:build integration

package eventbus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type syncReporter struct {
	mu  sync.Mutex
	got []usecase.ReportTaskOutcomeInput
}

func (s *syncReporter) Execute(_ context.Context, in usecase.ReportTaskOutcomeInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, in)
	return nil
}

func (s *syncReporter) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.got)
}

// Real JetStream on the TASK stream: events published while the consumer is down are delivered when it starts
// (durable), a task without a request is skipped, and a redelivery keeps the event id.
func TestTaskOutcomeConsumer_NATS_DurableDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "nats:2.10-alpine", Cmd: []string{"-js"}, ExposedPorts: []string{"4222/tcp"}, WaitingFor: wait.ForLog("Server is ready"),
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
	if err := pub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
		t.Fatal(err)
	}
	fixture := func(name string) json.RawMessage {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	mk := func(id, file string) commoneventbus.Event {
		return commoneventbus.Event{ID: id, TenantID: "11111111-1111-1111-1111-111111111111", OccurredAt: time.Now().UTC(), Version: 1, Payload: fixture(file)}
	}
	// Published before any consumer exists.
	for _, ev := range []commoneventbus.Event{mk("evt-1", "statuschanged_execute_claim.json"), mk("evt-2", "statuschanged_plain_task.json"), mk("evt-3", "statuschanged_execution_failed.json")} {
		if err := pub.Publish(ctx, "orca.task.task.statuschanged", ev); err != nil {
			t.Fatal(err)
		}
	}
	rep := &syncReporter{}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = NewTaskOutcomeConsumer(sub, rep).Run(runCtx) }()
	deadline := time.Now().Add(20 * time.Second)
	for rep.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	rep.mu.Lock()
	got := append([]usecase.ReportTaskOutcomeInput(nil), rep.got...)
	rep.mu.Unlock()
	if len(got) != 2 || got[0].EventID != "evt-1" || got[0].Cause != "execute_claim" || got[1].EventID != "evt-3" || got[1].ErrorMessage != "agent crashed" || got[1].RequestID != "req-1" {
		t.Fatalf("got %+v: the claim and the failure arrive (in order), the request-less task does not", got)
	}
	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop with its context")
	}
}

// The status consumer and the approval-gate consumer share the REQUEST stream with other consumers.
func TestExecutionConsumers_NATS_RequestStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "nats:2.10-alpine", Cmd: []string{"-js"}, ExposedPorts: []string{"4222/tcp"}, WaitingFor: wait.ForLog("Server is ready"),
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
	starter := &recordingStarter{}
	resumer := &recordingResumer{}
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { _ = NewRequestStatusConsumer(sub, starter).Run(runCtx) }()
	go func() { _ = NewApprovalDecidedConsumer(sub, resumer).Run(runCtx) }()
	tenantID := "11111111-1111-1111-1111-111111111111"
	status := func(id, to string) commoneventbus.Event {
		p, _ := json.Marshal(map[string]any{"request_id": "req-" + id, "to": to})
		return commoneventbus.Event{ID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: p}
	}
	decided := func(id, subject, decision string) commoneventbus.Event {
		p, _ := json.Marshal(domain.ApprovalDecidedPayload{RequestID: "req-" + id, SubjectType: subject, Decision: decision})
		return commoneventbus.Event{ID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: p}
	}
	for _, ev := range []commoneventbus.Event{status("a", "analyzing"), status("b", "executing")} {
		if err := pub.Publish(ctx, domain.SubjectRequestStatusChanged, ev); err != nil {
			t.Fatal(err)
		}
	}
	for _, ev := range []commoneventbus.Event{decided("c", "phase", "approved"), decided("d", "pre_deploy", "approved")} {
		if err := pub.Publish(ctx, domain.SubjectApprovalDecided, ev); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && (len(starter.started()) < 1 || len(resumer.resumed()) < 1) {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	if len(starter.started()) != 1 || starter.started()[0] != "req-b" {
		t.Fatalf("only the executing event starts: %v", starter.started())
	}
	if len(resumer.resumed()) != 1 || resumer.resumed()[0] != "req-d/pre_deploy" {
		t.Fatalf("only the approved pre_deploy resumes: %v", resumer.resumed())
	}
}
