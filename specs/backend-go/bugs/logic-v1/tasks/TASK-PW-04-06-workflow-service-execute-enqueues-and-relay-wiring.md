# TASK-PW-04-06: `Execute.runToCompletion` enqueues `workflow.execution.completed`/`.failed`; `main.go` relay wiring

**From Solution:** SOL-PW-04
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/execute.go`, `backend-go/services/workflow-service/cmd/server/main.go`
**Depends on:** TASK-PW-04-05
**Status:** `[x]` DONE — Execute.runToCompletion + RecoverExecutions.finish both enqueue orca.workflow.execution.completed/.failed; main.go wires EnsureStream("WORKFLOW", ...) + outbox.Relay; TestExecute_DispatchesWavesAndMarksExecutionCompleted/_WaveFailureMarksExecutionFailed assert the enqueued subject

---

## Context

**Grounding correction versus SOL-PW-04's own prose**: SOL-PW-04 names a
`workflow.proto` `ExecutionEvent`/`StreamExecutionEvents` type as needing
a `step_type` field for Flow 3's ref-sync filter — **that type does not
exist in `workflow.proto` today** (verified by reading the full RPC/
message list: only `WorkflowExecution{id,template_id,status,
root_trace_id,project_id}` exists, no per-step wire type). This task
therefore publishes only execution-level fields, matching what
`WorkflowExecution` actually carries — the step-level "is this a
create-pr-shaped step" filter Flow 3 needs is **out of scope for this
task** and is called out explicitly in TASK-PW-04-07 rather than
implemented against a type that doesn't exist.

`runToCompletion` (`execute.go:132-142`) is the one place `exec.Status`
transitions to a terminal value for the main dispatch path — the single
outbox publish point this task adds to.

## Changes to make

In `internal/usecase/execute.go`:

```go
func (uc *Execute) runToCompletion(ctx context.Context, exec domain.WorkflowExecution, waves [][]domain.Step) {
	succeeded := uc.dispatcher.dispatchWaves(ctx, exec.ID, waves)

	exec.Status = domain.StatusCompleted
	subject := "orca.workflow.execution.completed"
	if !succeeded {
		exec.Status = domain.StatusFailed
		subject = "orca.workflow.execution.failed"
	}

	payload, err := json.Marshal(workflowExecutionTerminalPayload{
		ExecutionID: exec.ID, TemplateID: exec.TemplateID, ProjectID: exec.ProjectID, Status: string(exec.Status),
	})
	var event *domain.OutboxEvent
	if err != nil {
		slog.ErrorContext(ctx, "workflow: marshaling terminal-status event payload failed", slog.String("execution_id", exec.ID), slog.Any("error", err))
	} else {
		event = &domain.OutboxEvent{ID: uuid.NewString(), Subject: subject, OccurredAt: time.Now().UTC(), PayloadJSON: payload}
	}

	if err := uc.executions.UpdateExecution(ctx, exec, event); err != nil {
		slog.ErrorContext(ctx, "workflow: persisting final execution status failed", slog.String("execution_id", exec.ID), slog.String("status", string(exec.Status)), slog.Any("error", err))
	}
}

type workflowExecutionTerminalPayload struct {
	ExecutionID string `json:"execution_id"`
	TemplateID  string `json:"template_id"`
	ProjectID   string `json:"project_id"`
	Status      string `json:"status"`
}
```

A marshal failure degrades to "persist status, skip the event" rather
than failing the whole terminal transition — matches this function's
existing best-effort logging posture for `UpdateExecution` failures
(both are already fire-and-forget from a background goroutine with no
caller to propagate an error to).

Also check `recover_executions.go:184-188` — it sets `exec.Status =
domain.StatusCompleted` and calls `UpdateExecution` for the boot-time
recovery path. Decide (and document the decision in this task's PR,
don't leave it implicit) whether a recovered execution should also
publish `orca.workflow.execution.completed` — the honest answer is
probably yes, since a downstream consumer that missed the original event
(e.g. was down) still needs to learn the execution finished. If yes,
apply the identical event-construction snippet there too.

Wire the relay in `cmd/server/main.go`, identical shape to
TASK-PW-04-04's `task-service` wiring:

```go
var relay *outbox.Relay
pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
if err != nil {
	logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
} else {
	defer func() { _ = closeBus() }()
	// Stream name "WORKFLOW" matches notification-service's ALREADY-WIRED
	// SubjectBinding{StreamName: "WORKFLOW", ...} — verified present in
	// services/notification-service/internal/adapter/eventbus/consumer.go
	// before picking this name; do not rename it.
	if err := pub.EnsureStream(ctx, "WORKFLOW", []string{"orca.workflow.>"}); err != nil {
		logger.WarnContext(ctx, "failed to ensure WORKFLOW stream", slog.Any("error", err))
	} else {
		relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
	}
}
if relay != nil {
	go relay.Run(ctx)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestExecute -v
```

Expected: clean build; a wave-dispatch success enqueues exactly one
`orca.workflow.execution.completed` event in the same transaction as the
status write; a failed wave enqueues `orca.workflow.execution.failed`
instead. Once deployed, `notification-service`'s already-existing
`orca.workflow.execution.completed`/`.failed` consumer and translation
rules (`internal/adapter/eventbus/consumer.go:46-47`,
`internal/domain/notification_event.go:102-109`) start receiving real
events with ZERO notification-service code change — confirm this
end-to-end, it's the whole point of this task existing.
