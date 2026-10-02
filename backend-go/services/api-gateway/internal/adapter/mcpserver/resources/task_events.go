package resources

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// EphemeralSubscriber is the slice of *commoneventbus.Consumer used here: a
// JetStream ephemeral consumer, so EVERY replica sees EVERY event.
type EphemeralSubscriber interface {
	SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error
}

type busSubject struct{ stream, subject string }

// Same closed set as task.activity.subscribe plus the status change event.
var taskSubjects = []busSubject{
	{"ORCHESTRATION", "orca.orchestration.task.dispatched"},
	{"ORCHESTRATION", "orca.orchestration.task.statuschanged"},
	{"ORCHESTRATION", "orca.orchestration.message.posted"},
	{"ORCHESTRATION", "orca.orchestration.decision_gate.opened"},
	{"WORKFLOW", "orca.workflow.step.completed"},
	{"WORKFLOW", "orca.workflow.step.failed"},
	{"TASK", "orca.task.agent_output_partial"},
	{"TASK", "orca.task.task.statuschanged"},
}

// BusTaskEvents adapts the event bus to TaskEventSource.
type BusTaskEvents struct{ Bus EphemeralSubscriber }

// Watch subscribes to every subject and blocks until ctx ends. Events without
// a tenant or task id are dropped (they could never be matched safely).
func (b BusTaskEvents) Watch(ctx context.Context, fn func(tenantID, taskID string)) error {
	var wg sync.WaitGroup
	for _, s := range taskSubjects {
		wg.Add(1)
		go func(s busSubject) {
			defer wg.Done()
			_ = b.Bus.SubscribeEphemeral(ctx, s.stream, s.subject, func(_ context.Context, ev commoneventbus.Event) error {
				var p struct {
					OriginTaskID string `json:"origin_task_id"`
					TaskID       string `json:"task_id"`
				}
				if err := json.Unmarshal(ev.Payload, &p); err != nil || ev.TenantID == "" {
					return nil
				}
				id := p.OriginTaskID
				if id == "" {
					id = p.TaskID
				}
				if id != "" {
					fn(ev.TenantID, strings.ToLower(id))
				}
				return nil
			})
		}(s)
	}
	wg.Wait()
	return nil
}
