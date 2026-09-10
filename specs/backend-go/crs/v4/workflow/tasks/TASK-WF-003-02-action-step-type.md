# TASK-WF-003-02: `action` step type

**From Solution:** BE-SOL-003
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`StepType` enum), `backend-go/services/workflow-service/internal/domain/step.go` (`StepTypeAction`, `ActionStepConfig`), `backend-go/services/workflow-service/internal/adapter/stepexecutors/` (new `action_executor.go` or similar, dispatch registry)
**Depends on:** None beyond BE-SOL-003's proto coordination note (shared `workflow.proto` edit with TASK-WF-003-01/-03 — land these together or in quick succession to avoid enum-value churn across concurrent branches)
**Status:** `[ ]` TODO

---

## Context

Re-verified directly: `StepType` (`internal/domain/step.go:16-23`) is a
closed 5-value enum — `StepTypeAgent | StepTypeShell |
StepTypeNotification | StepTypeWebhook | StepTypeCondition` — with a
`Valid()` switch (`:26-33`) that rejects anything else. The proto mirror
(`workflow.proto:53-60`) is the same 5-value `enum StepType`. Neither has
an `action` value today, confirmed.

`StepExecutorRegistry` (`internal/usecase/ports.go:93-100`) is a single
`Resolve(stepType domain.StepType) (domain.StepExecutor, error)` method —
adding a 6th `StepType` means adding a 6th registration in
`cmd/server/main.go`'s existing `registry.Register(...)` block
(`:94-99`), following the exact same pattern already used for the other
five.

## Changes to make

**1. `workflow.proto`** — add the enum value (append, don't renumber):

```protobuf
enum StepType {
  STEP_TYPE_UNSPECIFIED = 0;
  STEP_TYPE_AGENT = 1;
  STEP_TYPE_SHELL = 2;
  STEP_TYPE_NOTIFICATION = 3;
  STEP_TYPE_WEBHOOK = 4;
  STEP_TYPE_CONDITION = 5;
  STEP_TYPE_ACTION = 6; // NEW
}
```

**2. `internal/domain/step.go`**:

```go
const (
	StepTypeUnspecified  StepType = ""
	StepTypeAgent        StepType = "agent"
	StepTypeShell        StepType = "shell"
	StepTypeNotification StepType = "notification"
	StepTypeWebhook      StepType = "webhook"
	StepTypeCondition    StepType = "condition"
	StepTypeAction       StepType = "action" // NEW
)

func (t StepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeAction:
		return true
	default:
		return false
	}
}

// ActionStepConfig is the Action step type's config shape — dispatches to
// one of a registered set of action handlers (git.*, github.*, jira.*,
// ...) by name. See usecase's action dispatch registry for which handlers
// are actually wired — the set of supported actions is a product
// decision, not fixed by this type.
type ActionStepConfig struct {
	Action string         `json:"action"` // "git.createBranch" | "github.createPR" | ...
	Params map[string]any `json:"params"`
}
```

Also update the proto↔domain `StepType` conversion: confirmed live at
`internal/adapter/grpc/server.go:184-198`'s `toDomainStepType` — a plain
`switch` over the 5 existing proto enum values, `default` falling through
to `domain.StepTypeUnspecified`. Add a `case
workflowv1.StepType_STEP_TYPE_ACTION: return domain.StepTypeAction`
before the `default`. Also update `api-gateway`'s
`internal/adapter/wscompat/channels_automation_task.go:49`'s
`parseStepType(v string) workflowv1.StepType` helper — confirmed this is
the same string→enum parser `workflow.executeAdHocStep`'s wscompat channel
reuses (`channels_workflow.go:245`'s doc comment) — with an `"action"` ->
`STEP_TYPE_ACTION` case. Missing either of these is exactly the kind of
gap that compiles but silently mis-maps at runtime.

**3. Action dispatch registry** (new,
e.g. `internal/adapter/stepexecutors/action_executor.go`):

```go
// ActionExecutor dispatches ActionStepConfig.Action to a registered
// handler. The set of supported action names is intentionally small and
// explicit — see actionHandlers below — NOT a generic "call any RPC by
// string name" mechanism, to keep the attack surface (and the product
// surface) bounded.
type ActionExecutor struct {
	handlers map[string]ActionHandler
}

// ActionHandler executes one named action against its already-wired
// downstream client (git-gateway-service for git.*, issue-tracking-service
// for github.*/jira.*).
type ActionHandler func(ctx context.Context, params map[string]any) (domain.StepResult, error)

func NewActionExecutor(handlers map[string]ActionHandler) *ActionExecutor { ... }

func (e *ActionExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.ActionStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: action: invalid step config JSON: %w", err)
	}
	handler, ok := e.handlers[cfg.Action]
	if !ok {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: action: unregistered action %q", cfg.Action)
	}
	return handler(ctx, cfg.Params)
}
```

Reuse `git-gateway-service`'s and `issue-tracking-service`'s existing gRPC
clients — confirm at implementation time which of `workflow-service`'s
sibling services already dial these (or add a new dial in
`cmd/server/main.go`, following the exact `infrafleetclient.Dial` pattern
at `main.go:81-86`), and wire at minimum one real handler end to end
(whichever action the team greenlights first — **this is explicitly a
product decision, not fixed by this task**, per BE-SOL-003's own "Not in
scope" note).

**4. `cmd/server/main.go`** — register the new executor:

```go
registry.Register(domain.StepTypeAction, stepexecutors.NewActionExecutor(actionHandlers))
```

## Not in scope

- Which specific `action` handlers ship (which `git.*`/`github.*`/`jira.*`
  actions get real handlers) — product decision, per BE-SOL-003.
- UI for authoring `action` steps.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestStepType -v
go test ./services/workflow-service/internal/adapter/stepexecutors/... -run TestActionExecutor -v
```

Expected: `StepTypeAction.Valid()` returns true; an unregistered action
name fails clearly rather than panicking; at least one registered action
reaches its real downstream client in an integration test against a fake.
