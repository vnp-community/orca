# TASK-WF-002-03: Wire `ServerResolver`/`ProviderResolver` into the step executors

**From Solution:** BE-SOL-002
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `shell_step_executor.go`, `notification_step_executor.go`, `backend-go/services/workflow-service/cmd/server/main.go`
**Depends on:** TASK-WF-002-01 (`ServerResolver`), TASK-WF-002-02 (`ProviderResolver`), TASK-WF-001-01 (`agentExecParams`'s widened `Model`/`AccountID` fields)
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified live: all three executors matched the task's description
exactly (`cfg.ConnectionID` passed straight into `relay` as a pre-resolved
literal); no import cycle between `infrafleetclient` and `usecase` —
confirmed by a clean `go build` after `infrafleetclient` started importing
`usecase` for the two resolver types (`usecase` never imports
`infrafleetclient`, one-directional adapter→usecase dependency as
expected).

**Execution context plumbing — the real design decision this task
required — resolved with a THIRD option, not either of the two the task
sketched:** rather than widening `domain.StepExecutor.Execute`'s signature
(which every step type, including Condition/Webhook — neither has any
provider/server-resolution concept — would have to accept and ignore) or
coupling `wave_dispatcher.go` to agent-specific resolution logic, added
`usecase.ExecutionContext{ProjectID, TriggeredBy}` threaded via
`context.Context` — the same mechanism `common/tenant` already uses for
per-request identity (`tenant.RequireTenantID`/`tenant.UserID`). Stamped
once at each of the three places dispatch begins:
- `Execute.Execute`: captures `triggeredBy` from the still-live inbound
  ctx via `tenant.UserID` (before detaching to the background-goroutine
  ctx, which starts empty) + `exec.ProjectID`.
- `ExecuteAdHocStep.Execute`: `triggeredBy` from ctx (this path is
  synchronous, no detach); `ProjectID` stays empty — confirmed live that
  `ExecuteAdHocStepInput`/`Request` has no `ProjectID` field at all today,
  a real, separate gap out of this task's scope (ProviderResolver's chain
  and ai-provider-service's own server-scope tier already treat an empty
  ProjectID as valid, so this degrades gracefully, not silently wrong).
- `RecoverExecutions.recoverOne`: `exec.ProjectID` only — `TriggeredBy`
  is deliberately left empty (a boot-time recovery scan has no inbound
  request/acting user to capture, same reasoning as
  `ExecutionRepository.ListRunning`'s doc comment).

Read back via `usecase.ExecutionContextFrom(ctx)` inside `AgentExecutor.Execute`
only (the one executor that needs it) — `wave_dispatcher.go` itself is
completely unchanged, and Shell/Notification/Condition/Webhook executors
never see or care about this value. Documented the full reasoning in the
new `exec_context.go`'s doc comment per the task's explicit instruction to
note the choice.

**Changes made (widening + wiring, per the task's sketch):**
1. `agent_step_executor.go`: `AgentExecutor` widened with
   `serverResolver`/`providerResolver`; `Execute` now does
   `ParseTargetSpec` → `serverResolver.Resolve` → (via
   `ExecutionContextFrom(ctx)`) `providerResolver.Resolve` → `relay` with
   the resolved connection id + `Model`/`AccountID`.
2. `shell_step_executor.go` / `notification_step_executor.go`: same
   `ParseTargetSpec` → `serverResolver.Resolve` widening, no provider
   resolution (neither step type has that concept).
3. `internal/usecase/exec_context.go` (new): `ExecutionContext` + context
   helpers, as described above.
4. `execute.go` / `execute_ad_hoc_step.go` / `recover_executions.go`:
   stamp `ExecutionContext` at each dispatch-start point.
5. `cmd/server/main.go`: `serverResolver`/`providerResolver` (built in
   TASK-WF-002-01/-02, previously constructed-but-unused) now threaded
   into all three `New*Executor` calls; removed the placeholder
   `_, _ = serverResolver, providerResolver` blank-assignment.
6. Test fixtures: all three executors' existing tests updated —
   `"conn-1"` literals became `"server:conn-1"` (TargetKindServer's pure
   passthrough resolves back to the same literal, so existing assertions
   needed no other change) and constructors gained
   `newPassthroughServerResolver()`/`newNoopProviderResolver()` (new
   shared `resolver_fakes_test.go`, building REAL `*usecase.ServerResolver`/
   `*usecase.ProviderResolver` backed by fakes of their `ProjectClient`/
   `InfraFleetPicker`/`AIProviderClient` port dependencies — necessary
   because the executors now take concrete resolver structs, not
   interfaces, so the resolvers themselves can't be faked directly).
   Added 5 new tests per the task's test plan: malformed-target-spec
   surfaces as a clear error (not a panic) via `errors.Is`, end-to-end
   `project:`/`fleet:tag:` resolution reaching the right connection id,
   and an explicit provider pin reaching `Model`/`AccountID` in the relay
   params.

**Verify output:**
```
go build ./services/workflow-service/...   # clean, no import cycle
go vet   ./services/workflow-service/...   # clean
go test  ./services/workflow-service/internal/adapter/infrafleetclient/... -v
  # 16/16 PASS (11 pre-existing + 5 new: malformed spec, project-target,
  #             fleet-tag-target, explicit-pin-reaches-params)
go test  ./services/workflow-service/internal/usecase/... -run TestWaveDispatcher -v
  # 7/7 PASS — wave_dispatcher.go itself untouched, confirms the
  # context-threading approach needed zero changes there
go test  ./services/workflow-service/...   # full suite, all ok
```

---

## Context

Re-verified the exact current shape of all three executors and their
composition root wiring:

- `AgentExecutor`/`ShellExecutor`/`NotificationExecutor` (one struct each
  in their respective files) currently hold only `client
  infrafleetv1.InfraFleetServiceClient` and call `relay(ctx, e.client,
  cfg.ConnectionID, <method>, <params>, &result)` directly — `cfg.ConnectionID`
  is passed straight into `relay` as if it were already a resolved
  connection ID (confirmed: `agent_step_executor.go:58`,
  `shell_step_executor.go:47`, `notification_step_executor.go:67`).
- `cmd/server/main.go:94-99` constructs and registers all three via
  `stepexecutors.NewRegistry()` /
  `registry.Register(domain.StepTypeAgent,
  infrafleetclient.NewAgentExecutor(infraFleetClient))` (and the Shell/
  Notification equivalents), each taking only the one
  `infrafleetv1.InfraFleetServiceClient` constructor argument today.

This task widens all three executors' constructors to also take a
`*usecase.ServerResolver` (all three) and, for `AgentExecutor` only, a
`*usecase.ProviderResolver`, and updates `main.go`'s three `New*Executor`
calls accordingly.

**Layering note:** `internal/adapter/infrafleetclient` currently imports
only `domain` and the generated proto client — it does not import
`internal/usecase` today. Confirm at implementation time that
`usecase` importing `infrafleetclient` (for the `StepExecutorRegistry`
interface, `ports.go:98`) and `infrafleetclient` importing `usecase`'s
resolvers would not create an import cycle — per
`specs/backend-go/architecture/03-clean-architecture-guidelines.md`,
`usecase` should stay the layer that depends on `domain` and defines
ports; `infrafleetclient` (an adapter) is meant to depend inward on both
`domain` and `usecase`'s port/resolver types, which is the normal
adapter→usecase direction here, not a cycle — but re-check the actual
import graph with `go build` before assuming this compiles cleanly, since
`ServerResolver`/`ProviderResolver` are usecase-layer types being consumed
by an adapter-layer package.

## Changes to make

**1. `agent_step_executor.go`** — widen `AgentExecutor` and its
constructor, and use both resolvers in `Execute`:

```go
type AgentExecutor struct {
	client           infrafleetv1.InfraFleetServiceClient
	serverResolver   *usecase.ServerResolver
	providerResolver *usecase.ProviderResolver
}

func NewAgentExecutor(client infrafleetv1.InfraFleetServiceClient, serverResolver *usecase.ServerResolver, providerResolver *usecase.ProviderResolver) *AgentExecutor {
	return &AgentExecutor{client: client, serverResolver: serverResolver, providerResolver: providerResolver}
}

func (e *AgentExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.AgentStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: invalid step config JSON: %w", err)
	}

	spec, err := domain.ParseTargetSpec(cfg.ConnectionID) // field name unchanged; semantics widen from "literal ID" to "target spec" — see step.go's doc comment
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: parsing target spec: %w", err)
	}
	connectionID, err := e.serverResolver.Resolve(ctx, spec)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolving target: %w", err)
	}

	// projectID/triggeredBy: see "Execution context plumbing" below — not
	// yet available on stepConfigJSON's Execute signature as of this task.
	provider, err := e.providerResolver.Resolve(ctx, cfg, execProjectID, execTriggeredBy)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: resolving provider: %w", err)
	}

	var result execResult
	if err := relay(ctx, e.client, connectionID, agentExecMethod, agentExecParams{
		Prompt:       cfg.Prompt,
		WorktreePath: cfg.WorktreePath,
		TrustPreset:  cfg.TrustPreset,
		Model:        provider.Model,
		AccountID:    provider.AccountID,
	}, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: agent: %w", err)
	}

	return toStepResult(result)
}
```

**Execution context plumbing — a real gap this task must resolve, not
glossed over:** `StepExecutor.Execute`'s interface today is
`Execute(ctx context.Context, stepConfigJSON string) (StepResult, error)`
(`domain/step.go:104-106`) — it has no `projectID`/`triggeredBy` parameter
for `ProviderResolver.Resolve` to consume. Confirm at implementation time
where this execution context is available in `wave_dispatcher.go`'s
`runStep`/`dispatchStep` call chain (the execution row itself carries
`ProjectID`, per `usecase.ExecuteInput`'s doc comment) and either:

- widen `StepExecutor.Execute`'s signature to accept an execution-context
  value object (`{ProjectID, TriggeredBy string}`), threading it through
  `runStep`/`dispatchStep`/`dispatchWave`/`dispatchWavesFrom` — the
  larger, more invasive option but keeps `StepExecutor` self-contained; or
- have `wave_dispatcher.go` resolve the provider/server BEFORE calling
  `executor.Execute` and pass the resolved values in via the step config
  JSON it marshals — avoids touching the `StepExecutor` interface, but
  couples `wave_dispatcher.go` to agent-specific resolution logic it
  doesn't otherwise know about.

BE-SOL-002's own sketch glosses over this by writing `execCtx.ProjectID`/
`execCtx.TriggeredBy` as if an `execCtx` were already in scope inside
`agent_step_executor.go`'s `Execute` — it is not, today. Pick one of the
two options above (or propose a third) and note the choice in the PR
description; this is exactly the kind of design decision the parent
solution left implicit that an implementing engineer must make explicit.

**2. `shell_step_executor.go` / `notification_step_executor.go`** — same
`ServerResolver`-only widening (no `ProviderResolver` — neither step type
has a provider concept):

```go
type ShellExecutor struct {
	client         infrafleetv1.InfraFleetServiceClient
	serverResolver *usecase.ServerResolver
}

func NewShellExecutor(client infrafleetv1.InfraFleetServiceClient, serverResolver *usecase.ServerResolver) *ShellExecutor {
	return &ShellExecutor{client: client, serverResolver: serverResolver}
}
```

`Execute` gains the same `ParseTargetSpec` → `serverResolver.Resolve` two
lines before its existing `relay(...)` call, replacing the current
`cfg.ConnectionID` literal with the resolved `connectionID`. Identical
shape for `NotificationExecutor`.

**3. `cmd/server/main.go`** — construct the resolvers (per
TASK-WF-002-01/-02's wiring) before the registry block at line 94, then
update the three `New*Executor` calls:

```go
registry.Register(domain.StepTypeAgent, infrafleetclient.NewAgentExecutor(infraFleetClient, serverResolver, providerResolver))
registry.Register(domain.StepTypeShell, infrafleetclient.NewShellExecutor(infraFleetClient, serverResolver))
registry.Register(domain.StepTypeNotification, infrafleetclient.NewNotificationExecutor(infraFleetClient, serverResolver))
```

## Test plan

- Each executor's existing test suite updated to inject a fake
  `ServerResolver`/`ProviderResolver` (or fakes satisfying whatever ports
  those resolvers depend on) instead of relying on `cfg.ConnectionID`
  being pre-resolved.
- 2-step workflow targeting 2 different `server:` specs dispatches to the
  right connection each (per BE-SOL-002's end-to-end test plan item).
- A malformed `ConnectionID` (`ParseTargetSpec` error) surfaces as a
  clear step failure, not a panic or an opaque relay error.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go vet ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/infrafleetclient/... -v
go test ./services/workflow-service/internal/usecase/... -run TestWaveDispatcher -v
```

Expected: clean build with no import cycle between `infrafleetclient` and
`usecase`; all three executors' tests pass with resolvers injected;
`wave_dispatcher` end-to-end tests (or the chosen alternative from the
"Execution context plumbing" decision above) confirm resolved
`connectionID`/`Model`/`AccountID` reach the relay call correctly.
