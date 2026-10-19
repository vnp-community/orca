//go:build integration

package wscompat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
)

// taskServiceAgentOutputPartial is the exact payload task-service's SimpleExecutor stores in its outbox (see
// agentOutputPartialPayload there); both services' tests pin the same literal so a rename on either side fails.
const taskServiceAgentOutputPartial = `{"origin_task_id":"task-1","request_id":"req:r1:task-1:1","stdout":"· Session started (claude)\n▸ Read src/calc.py\n","stderr":""}`

// A frame published the way task-service's outbox relay does it (stream TASK, subject orca.task.agent_output_partial)
// reaches a task.activity.subscribe stream over a real NATS server, shaped as the web client reads it.
func TestTaskActivity_AgentOutputPartialOverRealNATS(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pub, cons, closeBus, err := commoneventbus.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closeBus() }()
	// Same stream definition as task-service's cmd/server/main.go.
	if err := pub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry()
	RegisterTaskActivityStreamChannel(registry, cons)
	sh, ok := registry.StreamHandlerFor("task.activity.subscribe")
	if !ok {
		t.Fatal("task.activity.subscribe not registered")
	}
	args, _ := json.Marshal(struct {
		TaskID string `json:"taskId"`
	}{TaskID: "task-1"})
	events, err := sh(ctx, Identity{TenantID: "tenant-1"}, []json.RawMessage{args})
	if err != nil {
		t.Fatal(err)
	}

	publish := func(id, tenantID, payload string) {
		t.Helper()
		ev := commoneventbus.Event{ID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: json.RawMessage(payload)}
		if err := pub.Publish(ctx, "orca.task.agent_output_partial", ev); err != nil {
			t.Fatal(err)
		}
	}
	publish("evt-foreign", "tenant-2", taskServiceAgentOutputPartial) // same task id, other tenant: must not arrive
	publish("evt-own", "tenant-1", taskServiceAgentOutputPartial)

	select {
	case ev := <-events:
		if ev.Channel != "task.activity.event" || len(ev.Args) != 1 {
			t.Fatalf("unexpected push %+v", ev)
		}
		raw, err := json.Marshal(ev.Args[0])
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			TaskID    string `json:"taskId"`
			Engine    string `json:"engine"`
			EventType string `json:"eventType"`
			Payload   struct {
				OriginTaskID string `json:"origin_task_id"`
				RequestID    string `json:"request_id"`
				Stdout       string `json:"stdout"`
				Stderr       string `json:"stderr"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.TaskID != "task-1" || wire.Engine != "direct_agent" || wire.EventType != "agent_output_partial" {
			t.Errorf("frame envelope = %+v", wire)
		}
		if wire.Payload.RequestID != "req:r1:task-1:1" || wire.Payload.Stdout != "· Session started (claude)\n▸ Read src/calc.py\n" {
			t.Errorf("frame payload = %+v", wire.Payload)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("no task.activity.event arrived over NATS")
	}
	select {
	case ev := <-events:
		t.Fatalf("another tenant's frame leaked: %+v", ev)
	case <-time.After(1500 * time.Millisecond):
	}
}
