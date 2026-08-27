# BUG-WF-02: DAG wave-dispatch and resumability are real, but server/provider resolution, variable interpolation, and live streaming are all missing

**Business Logic:** [BL-WF-02](../../../../docs/logic/workflow-orchestration/BL-WF-02-workflow-execution.md) — Multi-Server Workflow Execution
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A user can create a template with steps carrying a raw dev-server `connectionId` baked into each step's config, run it, and watch it fan out in parallel waves with per-step outcomes persisted to Postgres — and it survives a server restart mid-run. But they cannot: pass input variables to a run (`{{feature_description}}`), reference `{{outputs.stepId.field}}` from an earlier step in a later step's config, target a step by `"project:<id>"` / `"fleet:tag:<tag>"` and have the orchestrator resolve which dev server to use, have the orchestrator pick an AI provider account, use `action`/`parallel` step types, or watch step output stream live over WebSocket — they only get point-in-time polling via `GetExecution`.

---

## Spec summary

BL-WF-02 describes an orchestrator that: resolves a (possibly inherited) template, builds a DAG and topologically sorts it into parallel "waves," resolves each step's target server dynamically (`project:<id>` / `server:<id>` / `fleet:tag:<tag>` load-balancing) and AI provider account, executes 6 step types (`agent`, `shell`, `action`, `webhook`, `parallel`, `condition`), interpolates `{{...}}` variables from inputs/prior-step-outputs/project/user context, streams `step.output`/`step.completed`/`execution.completed` events live to the UI over WebSocket, persists state for resumability after a server restart, and supports a `parallel` step type with `allSettled`/`allowPartialFailure` semantics.

## What backend-go has

- Real DAG build + Kahn's-algorithm cycle detection + wave computation — `backend-go/services/workflow-service/internal/domain/dag.go:104-146`
- Bounded-concurrency wave dispatcher (max 10 concurrent steps/execution) that gates wave N+1 on wave N being fully terminal — `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go:26-38,109-166`
- `Execute` RPC: synchronous DAG validation + async background dispatch, `orca_workflow_executions`/`orca_step_executions`-equivalent persistence (`workflow.executions`, `workflow.step_executions`) — `backend-go/services/workflow-service/internal/usecase/execute.go:26-80`
- Boot-time recovery scan (`RecoverExecutions`) that re-attaches to every `status=running` execution, classifies each step's prior row (completed → skip, anything else → re-dispatch), and resumes from the correct wave — `backend-go/services/workflow-service/internal/usecase/recover_executions.go:11-40` (see full file for the wave-classification algorithm)
- 5 step types actually implemented: `agent`, `shell`, `notification` (relayed to infra-fleet-service's `Relay` RPC over a caller-supplied `ConnectionID`), `webhook` (native HTTP fetch), `condition` (pure in-memory expression evaluator) — `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `shell_step_executor.go`, `notification_step_executor.go`, `backend-go/services/workflow-service/internal/adapter/stepexecutors/webhook.go`, `condition.go`
- `PauseExecution`/`ResumeExecution`/`CancelExecution` RPCs, `HasActiveExecutions` for project-service's rebind guard — `backend-go/proto/orca/workflow/v1/workflow.proto:24-34,52`

## What's missing

- **No server resolution logic anywhere.** `AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig` all carry a raw `ConnectionID` field the caller must supply directly (`backend-go/services/workflow-service/internal/domain/step.go:60-89`). There is no `"project:<id>"` → project's bound dev server lookup, no `"server:<id>"` passthrough validation, and no `"fleet:tag:<tag>"` healthy-server load-balancing — confirmed absent by a repo-wide grep across `workflow-service`, `orchestration-service`, and `infra-fleet-service` for `fleet:tag`/`ServerResolver`/`resolveServer` (zero matches). This means the orchestrator itself does not do any cross-server dispatch decision-making — a workflow only spans multiple dev servers if the caller manually puts a different `ConnectionID` into each step's config ahead of time.
- **No AI provider resolution** (BL-AIP-02's priority-cascade, referenced by this spec as a delegation target). `AgentStepConfig` has `Prompt`/`WorktreePath`/`TrustPreset` only — no provider/model selection field, no cascading account lookup.
- **No variable interpolation at all.** `ExecuteRequest` (`workflow.proto:87-92`) has no `inputs` field — a caller cannot pass `{{feature_description}}`-style run-time inputs. There is no `interpolate()` equivalent anywhere in `workflow-service`; confirmed by the service's own README: "step outputs are not threaded into later steps' config as accumulated context... this pass's `ConditionExecutor` still only sees whatever `stepConfigJSON` the DAG step itself carries" (`backend-go/services/workflow-service/README.md:169-174`). `{{outputs.stepId.field}}`, `{{project.*}}`, `{{user.*}}`, `{{now()}}` are all unimplemented.
- **Only 5 of 6 step types exist; `action` and `parallel` are both entirely absent.** `StepType` enum is `agent | shell | notification | webhook | condition` (`workflow.proto:57-64`, `domain/step.go:17-23`) — no `action` type (generic action-executor dispatch) and, critically, **no `parallel` step type at all**, so the `Promise.allSettled` + `allowPartialFailure` sub-step fan-out the spec describes has no implementation to check (confirmed by grep: zero matches for `parallel`/`allowPartialFailure` anywhere in `workflow-service` or the proto). Wave-level parallelism (independent DAG steps in the same wave) does exist and is real, but that's a different mechanism from the spec's `parallel` step *type* nesting sub-steps.
- **No live streaming of execution events.** No `step.output`, `step.completed`, or `execution.completed` WebSocket events are emitted — confirmed absent by grep across `workflow-service` and `wscompat` (only the README's own admission that `StreamExecutionEvents` "isn't in the generated proto and so isn't implemented here," `README.md:75-76`). Clients must poll `GetExecution`/`GetStepExecution`-style reads; there is no push channel. Compounding this, `workflow.*` has zero `wscompat` channel registrations at all (`grep '"workflow\.' services/api-gateway/internal/adapter/wscompat/channels.go` → no matches) — see BUG-030.
- **Resumability's "running → retry from scratch" nuance differs from spec but is arguably safer**: spec says a step caught mid-`running` at restart is retried from scratch; backend-go's `RecoverExecutions` re-dispatches (via `UpdateStepExecution` on the same row) anything short of a recorded `completed`, which is functionally similar but not verified against the exact same semantics (no distinction between a step that failed vs. one that was merely running when the crash happened, both are retried — spec doesn't explicitly forbid this, so not counted as a hard gap, but flagged for completeness).

## See also

- `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` — `workflow.*` has no `wscompat` WS wrapper; REST (`Execute`/`CancelExecution`/`CreateTemplate`) already works. This bug is about the deeper gap: even with WS wired, there is nothing pushing `step.output`/`execution.completed` events to relay in the first place — no `StreamExecutionEvents` RPC exists to wrap.

## References

- `backend-go/docs/logic/workflow-orchestration/BL-WF-02-workflow-execution.md` — spec
- `backend-go/proto/orca/workflow/v1/workflow.proto:56-64,87-92` — `StepType` enum (5 values, no action/parallel) and `ExecuteRequest` (no `inputs` field)
- `backend-go/services/workflow-service/internal/domain/step.go:60-89` — `AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig`, each keyed by a caller-supplied `ConnectionID`
- `backend-go/services/workflow-service/internal/usecase/execute.go:16-25` — `ExecuteInput` (TemplateID/ProjectID/RootTraceID/RequestID only, no inputs)
- `backend-go/services/workflow-service/README.md:75-76,169-174` — documented gaps: no `StreamExecutionEvents`, no step-output threading into later steps
- `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go` — real wave-gated dispatch engine
- `backend-go/services/workflow-service/internal/usecase/recover_executions.go` — boot-time resumability scan
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — no `workflow.*` registrations (grep confirmed zero matches)
