# TASK-AT-03-05: Durable JetStream consumer subscribing to the 5 event subjects

**From Solution:** SOL-AT-03
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/eventbus/consumer.go` (new), `backend-go/services/automation-service/cmd/server/main.go`
**Depends on:** TASK-AT-03-04
**Status:** `[x]` DONE — adapter/eventbus/consumer.go: durable Subscribe (not ephemeral) over the 5 subjects, exhaustive subjectToEventName; wired into cmd/server/main.go alongside the outbox relay.

---

## Context

Wire a **durable** (not ephemeral) JetStream consumer — every replica
competes for the same named consumer, one delivery per event across the
fleet. Contrast with `notification-service`'s `SubscribeEphemeral`, which is
per-replica by design for a different reason (WS fan-out); using that here
would multiply-dispatch across replicas, the opposite bug.

## Changes to make

Create `backend-go/services/automation-service/internal/adapter/eventbus/consumer.go`:

```go
package eventbus

type SubjectBinding struct {
	StreamName string
	Subject    string
}

// Subjects maps BL-AT-03's 5 event names to real (or planned) subjects.
// worktree:created, pr:merged, and issue:assigned have no publisher yet —
// see SOL-AT-03's "Cross-service work needed" section; subscribing here is
// safe regardless (no events arrive until those publishers exist).
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},
	{StreamName: "TASK", Subject: "orca.task.task.failed"},
	{StreamName: "PROJECT", Subject: "orca.project.worktree.created"},
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.pull_request.merged"},
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.issue.assigned"},
}

func subjectToEventName(subject string) domain.EventName {
	switch subject {
	case "orca.task.task.completed":
		return domain.EventAgentCompleted
	case "orca.task.task.failed":
		return domain.EventAgentError
	case "orca.project.worktree.created":
		return domain.EventWorktreeCreated
	case "orca.scmintegration.pull_request.merged":
		return domain.EventPRMerged
	case "orca.scmintegration.issue.assigned":
		return domain.EventIssueAssigned
	default:
		panic("eventbus: unmapped subject " + subject) // exhaustiveness enforced by consumer_test.go
	}
}

type Consumer struct {
	bus      *commoneventbus.Consumer
	dispatch *usecase.HandleEventTrigger
}

func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	for _, b := range Subjects {
		b := b
		go func() {
			// Durable (NOT SubscribeEphemeral) — see rationale in SOL-AT-03.
			if err := c.bus.Subscribe(ctx, b.StreamName, b.Subject, func(ctx context.Context, event commoneventbus.Event) error {
				return c.dispatch.Execute(ctx, usecase.HandleEventTriggerInput{
					EventID: event.ID, TenantID: event.TenantID,
					EventName: subjectToEventName(b.Subject), Payload: event.Payload,
				})
			}); err != nil {
				logger.Error("automation eventbus subscribe failed", "error", err, "subject", b.Subject)
			}
		}()
	}
}
```

Check `commoneventbus.Consumer`'s actual `Subscribe` signature (durable vs.
ephemeral mode may be a parameter rather than a separate method) before
wiring — use whatever durable-mode API this shared package already exposes.

Wire `Consumer` into `cmd/server/main.go` alongside this service's other
startup wiring (repository, usecases, gRPC server) — construct
`HandleEventTrigger` with the real repository/`RunNow` and call `Run` in a
goroutine before the server blocks on its listener.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/adapter/eventbus/...
```

Expected: `consumer_test.go` — subject→`EventName` mapping table is
exhaustive over the 5 documented names (a test driving every entry in
`Subjects` through `subjectToEventName` without panicking); an integration
test (if this service's suite runs a real/embedded JetStream) confirming
exactly one of N replica instances processes a given event.
