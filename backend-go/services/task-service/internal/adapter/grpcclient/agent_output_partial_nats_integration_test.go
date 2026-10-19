//go:build integration

package grpcclient

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/common/testutil"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// memoryOutbox is the repository's outbox table in memory: SimpleExecutor writes it, the real outbox relay reads it.
type memoryOutbox struct {
	mu   sync.Mutex
	rows []outbox.Record
	done map[string]bool
}

func (m *memoryOutbox) InsertOutboxEvent(_ context.Context, id, tenantID, subject string, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, outbox.Record{ID: id, Subject: subject, Event: commoneventbus.Event{
		ID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	}})
	return nil
}

func (m *memoryOutbox) FetchUnpublished(_ context.Context, limit int) ([]outbox.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []outbox.Record
	for _, r := range m.rows {
		if !m.done[r.ID] && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memoryOutbox) MarkPublished(_ context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done == nil {
		m.done = map[string]bool{}
	}
	for _, id := range ids {
		m.done[id] = true
	}
	return nil
}

var (
	_ usecase.OutboxWriter = (*memoryOutbox)(nil)
	_ outbox.Store         = (*memoryOutbox)(nil)
)

// Output streamed by the agent travels SimpleExecutor -> outbox -> relay -> NATS stream TASK on the subject
// api-gateway's task.activity.subscribe reads, with the activity lines (· » ▸ ↳ ■) intact in the cumulative frame.
func TestAgentOutputPartial_ReachesNATSThroughTheOutboxRelay(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// The mapped port can refuse the first few connects right after the container reports ready.
	var pub *commoneventbus.Publisher
	var cons *commoneventbus.Consumer
	var closeBus func() error
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		if pub, cons, closeBus, err = commoneventbus.Connect(ctx, url); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	// Same stream definition and relay construction as cmd/server/main.go.
	if err := pub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
		t.Fatal(err)
	}
	store := &memoryOutbox{}
	relay := outbox.NewRelay(store, pub, outbox.Config{PollInterval: 50 * time.Millisecond, BatchSize: 10}, nil)
	go relay.Run(ctx)

	received := make(chan commoneventbus.Event, 8)
	go func() {
		_ = cons.SubscribeEphemeral(ctx, "TASK", agentOutputPartialSubject, func(_ context.Context, ev commoneventbus.Event) error {
			received <- ev
			return nil
		})
	}()

	tasks := &fakeTaskRepository{tasks: map[string]domain.Task{"t1": {ID: "t1", ProjectID: "p1"}}}
	resolver := &fakeProjectExecutionResolver{worktreePath: "/srv/worktrees/p1", devServerID: "ds-1", connected: true}
	relayClient := &fakeInfraFleetServiceClient{
		relayByDevServerResp: &infrafleetv1.RelayResponse{ResultJson: `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`},
	}
	lines := []string{"· Session started (claude)\n", "» I'll read the file.\n", "▸ Read src/calc.py\n", "↳ ok · 3 lines\n", "■ Done · 2 turns · 5.9s\n"}
	chunks := make(chan usecase.AgentExecOutputChunk, len(lines))
	for _, l := range lines {
		chunks <- usecase.AgentExecOutputChunk{Stream: "stdout", Data: l}
	}
	close(chunks)
	exec := NewSimpleExecutor(tasks, &fakeEdgeRepository{}, resolver, relayClient, &fakeSimpleExecutorProfileResolver{}, &fakeSimpleExecutorProjectContextResolver{}, store, &fakeAgentExecOutputStreamer{chunks: chunks})
	exec.throttleInterval = time.Hour // only the final flush publishes: one cumulative frame

	if _, err := exec.Execute(ctxWithTenant(t), "tenant-1", "t1", "req-1", "/srv/worktrees/p1", ""); err != nil {
		t.Fatalf("execute: %v", err)
	}

	select {
	case ev := <-received:
		if ev.TenantID != "tenant-1" {
			t.Errorf("tenant = %q, want tenant-1", ev.TenantID)
		}
		var p struct {
			OriginTaskID string `json:"origin_task_id"`
			RequestID    string `json:"request_id"`
			Stdout       string `json:"stdout"`
			Stderr       string `json:"stderr"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		want := ""
		for _, l := range lines {
			want += l
		}
		if p.OriginTaskID != "t1" || p.RequestID != "req-1" || p.Stdout != want {
			t.Errorf("payload = %+v, want origin t1, request req-1, stdout %q", p, want)
		}
		if _, err := uuid.Parse(ev.ID); err != nil {
			t.Errorf("event id %q is not a uuid: %v", ev.ID, err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no agent_output_partial frame reached NATS")
	}
}
