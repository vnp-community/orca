# SOL-AG-TG-001: `agent.execPrompt` env-injection contract — assessment, zero code change required

> **📐 Assessment-only — confirms no `agent/` code change is needed for
> [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md)'s
> env-injection fix.** The fix lives entirely in `task-service`'s
> `SimpleExecutor` — see
> [BE-SOL-005](../../../../../backend-go/crs/v4/task-graph/solutions/BE-SOL-005-task-agent-execution-permission-and-complex-executor.md).
> This document exists so that assessment isn't taken on faith — it reads
> `agent-print-mode-exec.ts` in full and shows exactly why the existing
> contract already suffices.

**CR:** [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md) §2.4 ("env injection")
**Depends on:** nothing — reads current code only
**Affected files:** none

---

## 1. What CR-TG-005 needs from `agent/`

`task-service.SimpleExecutor` sends `StepID: requestID` (not the real
`taskID`) to `agent.execPrompt` today, and has no `Env` field on its params
struct at all. CR-TG-005's fix is to add `Env: {"ORCA_TASK_ID": taskID,
"ORCA_PROJECT_ID": task.ProjectID}` on the backend-go side. The open
question this document answers: **does the agent-side handler actually
honor an explicit `env.ORCA_TASK_ID`, or does it always overwrite it with
its own derived value from `stepId`?**

## 2. Reading `agent-print-mode-exec.ts` end to end

```typescript
// agent-print-mode-exec.ts:47-56
const extraEnv =
  params.env && typeof params.env === 'object' && !Array.isArray(params.env)
    ? (params.env as Record<string, string>)
    : undefined
// ...
env = await buildAgentEnv(
  { accountId, userId: '', taskId: stepId ?? '', cwd: worktreePath, model: modelId, extraEnv },
  spec, config, null, log, span.id
)
// ...
const child = spawn(spec.binary, args, {
  cwd: worktreePath,
  env: { ...process.env, ...env },   // <-- env is buildAgentEnv's fully-merged return value
  stdio: ['ignore', 'pipe', 'pipe']
})
```

`params.env` is already read (`extraEnv`) and passed into `buildAgentEnv`,
whose own doc comment at this call site states explicitly: *"ProfileAwareAgentSpawner
forwards profile-resolved env (PATH additions, `ORCA_PROJECT_ID`/
`ORCA_ACCOUNT_ID`/etc.) here — merged **on top of** `buildAgentEnv()`'s base
env via its own `extraEnv` slot, same override order `agent.spawn` already
uses"* (`agent-print-mode-exec.ts:47-50`). "Merged on top of" means
`extraEnv` wins over whatever `buildAgentEnv` derives internally from
`taskId: stepId ?? ''` — confirmed by cross-checking `12-agent-spawner.md`'s
description of the same `buildAgentEnv` function, which documents
`extraEnv` as the last-applied layer.

**Conclusion:** a backend-go caller that sets `params.env = {ORCA_TASK_ID:
"&lt;real task id&gt;", ORCA_PROJECT_ID: "&lt;project id&gt;"}` will have those
values reach the spawned process's environment, correctly overriding the
otherwise-wrong `ORCA_TASK_ID` derived from `stepId`/`requestID`. No change
to `handleAgentExecPrompt`, `buildAgentEnv`, or any other `agent/` file is
required.

## 3. Incidental finding, out of scope for CR-TG-005

`buildAgentEnv` is called with `userId: ''` unconditionally in this
handler (`agent-print-mode-exec.ts:109`) — meaning `ORCA_USER_ID` (if
`buildAgentEnv` sets one) is always empty for the `agent.execPrompt` path,
regardless of any `env` override, since `userId` isn't part of `extraEnv`'s
override surface, it's a separate positional field. This wasn't part of
CR-TG-005's scope (which only asked about `ORCA_TASK_ID`/`ORCA_PROJECT_ID`)
— flagged here for whoever picks up user-identity propagation next, not
acted on in this assessment.

## Not in scope

- Any design work — this is a confirmation, not a solution to implement.
- The incidental `userId: ''` finding in §3 — needs its own CR if the
  product wants `ORCA_USER_ID` propagated through this path.

## References

- [CR-TG-005](../../../../../../docs/crs/v4/task-graph/CR-TG-005-task-agent-execution-permission-and-complex-executor.md)
- `agent/src/relay/agent-print-mode-exec.ts:33-178`
- `specs/agent/tdd/v5/12-agent-spawner.md`
