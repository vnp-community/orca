# workflow (F36) tasks — index

Executable task breakdown of the `workflow` CR series' solutions
([`../solutions/`](../solutions/)). Every task below is `Status: [ ] TODO`
(one exception — `TASK-WF-006-01` is `[ ] BLOCKED`, see below) — none of
this is implemented yet. Each task cites real, current `backend-go`
file:line locations (re-verified against the on-disk source while writing
these tasks, not copied blind from the solutions) and calls out every
place a solution's sketch didn't match the real code, so an implementing
agent can work from the task file alone without re-reading the parent
solution end to end.

## Solution → Task ID map

| Solution | Task IDs | Notes |
|---|---|---|
| [BE-SOL-001](../solutions/BE-SOL-001-fix-agent-step-executor-relay-method.md) — fix `AgentExecutor`'s relay method | `TASK-WF-001-01` | Smallest task in the series — the bug and its fix are already documented in the file's own doc comment, just not applied. Verified byte-for-byte against the live file. |
| [BE-SOL-002](../solutions/BE-SOL-002-server-and-provider-resolution.md) — `TargetSpec`/`ServerResolver`, `ProviderResolver` | `TASK-WF-002-01` (`TargetSpec` + `ServerResolver`), `TASK-WF-002-02` (`ProviderResolver`), `TASK-WF-002-03` (wiring into step executors), `TASK-WF-002-04` (**new, not a BE-SOL-002 task** — `infra-fleet-service`'s missing `PickByTag` RPC, promoted from a blocking note in `TASK-WF-002-01` into its own trackable task) | `TASK-WF-002-01` flags a real, unresolved gap: `infra-fleet-service` has no `PickByTag` RPC and no equivalent selection usecase to reuse — confirmed live, this is a genuine new-RPC requirement, not optional scope; `TASK-WF-002-04` is that fix. `TASK-WF-002-02` corrects BE-SOL-002's `ResolveForContext` RPC name (the real RPC is `ResolveProvider`) and its `proj.GetDevServerId()` accessor (the real accessor is `resp.GetProject().GetDevServerId()`, nested one level deeper than the solution's sketch). `TASK-WF-002-03` surfaces a real, unresolved design gap BE-SOL-002's sketch glossed over — `StepExecutor.Execute`'s interface has no execution-context (`projectID`/`triggeredBy`) parameter for `ProviderResolver` to consume; two options are given, neither picked. |
| [BE-SOL-003](../solutions/BE-SOL-003-variable-interpolation-and-step-types.md) — `ExecuteRequest.inputs`, interpolation, `action`/`parallel` step types | `TASK-WF-003-01` (inputs + interpolation), `TASK-WF-003-02` (`action` step type), `TASK-WF-003-03` (`parallel` step type) | `TASK-WF-003-01` carries BE-SOL-003's own coordination flag forward: `ExecuteRequest` is also touched by `CR-FLOW-TASK-002`'s `origin_task_id` field — confirmed no collision exists yet (`workflow.proto`'s `ExecuteRequest` is still the original 4-field shape), but the task instructs a live re-check of field numbers before editing. `TASK-WF-003-01`/`-03` correct BE-SOL-003's `executeStep()` function-name sketch — the real dispatch function is `runStep` (`wave_dispatcher.go:199-214`), no `executeStep` exists. |
| [BE-SOL-004](../solutions/BE-SOL-004-template-inheritance-merge-and-clone.md) — deep-merge, `CloneTemplate`, conditional version bump | `TASK-WF-004-01` (overrides/injectSteps/removeSteps), `TASK-WF-004-02` (`CloneTemplate` RPC), `TASK-WF-004-03` (conditional version bump) | `TASK-WF-004-01` re-states, and the implementation must preserve, `resolveEffectiveTemplate`'s existing "closest-with-steps-wins" base-selection policy — verified live, unchanged by this task, only extended. It also flags that `domain.DAGDefinition` has no `Serialize()` method today (BE-SOL-004's sketch assumes one) — this task adds it. `TASK-WF-004-03` corrects a real divergence: BE-SOL-004 sketches a nonexistent `UpdateConditional(...)` repository method; the real method is `TemplateRepository.Update(ctx, tmpl, expectedVersion)`, widened here with a new `bump bool` parameter instead. Flags the "what counts as breaking" product-decision requirement explicitly. |
| [BE-SOL-005](../solutions/BE-SOL-005-template-sharing-library-and-list-executions.md) — sharing/library schema+RPCs, `ListExecutions` fix | `TASK-WF-005-01` (`ListExecutions`, **P0** — production bug fix), `TASK-WF-005-02` (schema widening + `UpdateVisibility`/`GenerateShareLink`/`GetTemplateByShareToken`), `TASK-WF-005-03` (`SearchTemplates`/`RateTemplate`/`usage_count`, import-via-Clone) | `TASK-WF-005-01` is prioritized ahead of every other task in BE-SOL-005 (and given P0, matching this series' highest priority alongside `TASK-WF-001-01`) — `WorkflowMonitor.tsx` calls `workflow.listExecutions`, which does not exist on backend-go today; confirmed live (11 registered wscompat channels, `listExecutions` not among them), so the Workflow tab is broken in production for every backend-go user right now. `TASK-WF-005-02` carries an explicit, non-optional security-review requirement (unauthenticated `GetTemplateByShareToken` path) forward from the solution. |
| [BE-SOL-006](../solutions/BE-SOL-006-execution-live-streaming.md) — publish `orca.workflow.step.completed`/`.failed` | `TASK-WF-006-01` — `[ ] BLOCKED` | See "Why `TASK-WF-006-01` exists as a BLOCKED task, not an omitted one" below. |

## Dependency order

```
TASK-WF-001-01 (fix agent.exec → agent.execPrompt, widen agentExecParams)
      │
      ▼
TASK-WF-002-01 (TargetSpec + ServerResolver)          TASK-WF-002-02 (ProviderResolver)
      │  ⚠ needs TASK-WF-002-04 for the                     │  independent of -01
      │    fleet:tag: branch specifically —
      │    project:/server: branches don't
      │    need it, can land without waiting
TASK-WF-002-04 (infra-fleet-service PickByTag RPC —
                independent, own service, land any time)
      └──────────────────────┬─────────────────────────────┘
                              ▼
                  TASK-WF-002-03 (wire both resolvers into
                  agent/shell/notification step executors —
                  depends on -01, -02, AND TASK-WF-001-01's
                  widened agentExecParams shape)
                              │
                              ▼
                  TASK-WF-003-01 (ExecuteRequest.inputs +
                  interpolation pass — depends on -002-03 since
                  interpolation runs on step config JSON before
                  the resolvers/executors consume it)
                              │
              ┌───────────────┼───────────────┐
              ▼                               ▼
  TASK-WF-003-02 (action step type)   TASK-WF-003-03 (parallel step type)
  (shares workflow.proto StepType enum edit — land together or in quick succession)


TASK-WF-004-01 (deep-merge: overrides/injectSteps/removeSteps)  — independent, can run in parallel with 001/002/003
TASK-WF-004-02 (CloneTemplate RPC)                              — independent of -004-01
TASK-WF-004-03 (conditional version bump)                       — independent of -004-01/-02
      │
      ▼ (004-02's CloneTemplate is 005-03's reuse target)
TASK-WF-005-01 (ListExecutions fix — P0, independent of everything else, ship first if possible)
TASK-WF-005-02 (sharing schema + UpdateVisibility/GenerateShareLink/GetTemplateByShareToken — needs security review)
      │
      ▼
TASK-WF-005-03 (SearchTemplates/RateTemplate/usage_count —
                depends on -005-02's schema AND TASK-WF-004-02's CloneTemplate for "Import to My Workflows")

TASK-WF-006-01 [BLOCKED] — hard-blocked on CR-FLOW-TASK-003 (and
                CR-FLOW-TASK-002's origin_task_id) landing first,
                independent of every task above
```

This mirrors [`../solutions/README.md`](../solutions/README.md)'s own
dependency note: BE-SOL-001 → BE-SOL-002 → BE-SOL-003 is sequential;
BE-SOL-004 is independent and can run in parallel with 001-003;
BE-SOL-005 depends on BE-SOL-004 (Clone) and benefits from splitting its
`ListExecutions` fix out as an urgent fast-follow (`TASK-WF-005-01`);
BE-SOL-006 is blocked on an entirely separate CR series
(`CR-FLOW-TASK-003`) and does not gate, or get gated by, anything else in
this directory.

## Why `TASK-WF-006-01` exists as a BLOCKED task, not an omitted one

`specs/backend-go/crs/v3/flow-task/tasks/README.md`'s own precedent (its
"Why BE-SOL-004 has no tasks" section) omits task files entirely for a
solution whose own content is "don't build anything yet," with no
concrete design underneath to break down — that solution is a pure
readiness confirmation, proposing literally zero code change.

BE-SOL-006 is a different shape: it specifies a fully concrete, small,
well-defined change — one publish call site in `wave_dispatcher.go`, one
new channel-key convention for executions with no `OriginTaskID` — that
simply cannot compile or be tested today because its dependency
(`CR-FLOW-TASK-003`'s outbox/event-catalog machinery, plus
`CR-FLOW-TASK-002`'s `origin_task_id` field) doesn't exist in the
codebase yet. This is closer to the shape of
`specs/backend-go/bugs/logic-v1/tasks/TASK-AG-04-05-blocked-switch-inherits-credential-injection-gap.md`
— a real task file exists, with `Status: [ ] BLOCKED` and an explicit
"needs X first" note naming the exact blocker — rather than
`specs/backend-go/crs/v4/workflow/solutions/`'s BE-SOL-004-in-the-other-series
precedent of no task file at all.

**Why a task file here, and not just a note in this README** (as the
other precedent does): the design is concrete enough that writing it down
now saves whoever unblocks this from re-deriving it from BE-SOL-006 cold
— exact call site, exact event names, exact channel-key fallback
behavior are all already decided. A README note alone would lose that.
The `[ ] BLOCKED` status (not `[ ] TODO`) is what prevents an
implementing agent from picking this up prematurely — see
`TASK-WF-006-01`'s own "Unblock checklist" for the exact two conditions
that must both be confirmed `DONE` (via `specs/backend-go/crs/v3/flow-task/tasks/README.md`'s
task statuses) before flipping its status.

## Cross-references — do not re-derive elsewhere

- [`../solutions/README.md`](../solutions/README.md) — the parent
  solutions' own dependency ordering and status table.
- [`docs/crs/v3/flow-task/`](../../../../../../docs/crs/v3/flow-task/) —
  Task↔Workflow linkage (`CR-FLOW-TASK-002`, `origin_task_id`), unified
  event catalog (`CR-FLOW-TASK-003`, `TASK-WF-006-01`'s hard blocker).
- [`specs/backend-go/crs/v3/flow-task/tasks/`](../../../../crs/v3/flow-task/tasks/) —
  the task breakdown for the CR series `TASK-WF-006-01` is blocked on;
  check its `README.md`'s task statuses before flipping `TASK-WF-006-01`
  to `TODO`.
- [`docs/crs/v4/workflow/`](../../../../../../docs/crs/v4/workflow/) —
  the CRs each solution in this directory resolves.
