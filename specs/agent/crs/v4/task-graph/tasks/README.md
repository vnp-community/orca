# Agent Tasks — Task Graph (v4)

**Solutions:** [../solutions/](../solutions/)

Executable task breakdown of the `task-graph` CR series' `agent/`-side solutions. Both task
files below were written after re-reading the real current source
(`agent/src/relay/agent-git-handler.ts`'s `handleGitExecStream`,
`agent/src/relay/agent-rpc-dispatch-git.ts`'s `case 'git.execStream'`,
`agent/src/relay/agent-print-mode-exec.ts`'s `handleAgentExecPrompt`,
`agent/src/relay/fs-agent-extensions.ts`'s `handleShellExec`) — not copied blind from the
solution docs — and each task calls out the places that re-verification diverged from the parent
solution's sketch.

## Solution → Task ID map

| Solution | Task IDs | Notes |
|---|---|---|
| [SOL-AG-TG-001](../solutions/SOL-AG-TG-001-agent-env-contract-assessment.md) — `agent.execPrompt` env-injection contract assessment | **None** | See "Why SOL-AG-TG-001 has no tasks" below. |
| [SOL-AG-TG-002](../solutions/SOL-AG-TG-002-agent-chunk-streaming.md) — `agent.execPromptStream`/`shell.execStream` chunk streaming | [TASK-AG-TG-001](./TASK-AG-TG-001-agent-exec-prompt-stream-handler.md) (`handleAgentExecPromptStream` + `agent.execPromptStream` dispatch), [TASK-AG-TG-002](./TASK-AG-TG-002-shell-exec-stream-handler.md) (`handleShellExecStream` + `shell.execStream` dispatch) | Both tasks reuse the exact same real precedent (`git.execStream`); see each task's Context for the file-specific complications re-reading the code surfaced. |

## Why SOL-AG-TG-001 has no tasks

SOL-AG-TG-001 is explicitly an assessment, not a design — its own header states "confirms no
`agent/` code change is needed" and its "Not in scope" section says plainly: "Any design work —
this is a confirmation, not a solution to implement." It reads `agent-print-mode-exec.ts` end to
end and shows that `params.env`'s `extraEnv` already overrides `buildAgentEnv`'s internally
derived `ORCA_TASK_ID`, so CR-TG-005's env-injection fix requires zero `agent/` change — the
entire fix is `task-service`'s `SimpleExecutor` (see
[BE-SOL-005](../../../../../backend-go/crs/v4/task-graph/solutions/BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)).
Creating an implementation task for a solution whose entire content is "no code change required"
would be inventing work the solution itself says isn't there. (This mirrors the precedent in
`specs/backend-go/crs/v3/flow-task/tasks/README.md`'s "Why BE-SOL-004 has no tasks" section for
an analogous confirmation-only solution.)

The one incidental finding in SOL-AG-TG-001 §3 (`buildAgentEnv` is called with `userId: ''`
unconditionally in `handleAgentExecPrompt`, so `ORCA_USER_ID` is always empty on this path) is
explicitly flagged as out of scope for CR-TG-005 and needs its own CR before it becomes a task —
not turned into a task here.

## Dependency order

```
SOL-AG-TG-001 — no tasks (assessment only, zero code change)

SOL-AG-TG-002
  ├── TASK-AG-TG-001 (agent.execPromptStream — agent-print-mode-exec.ts,
  │                    agent-rpc-dispatch-agent-exec.ts)
  └── TASK-AG-TG-002 (shell.execStream — fs-agent-extensions.ts,
                       agent-rpc-dispatch-misc.ts)
```

TASK-AG-TG-001 and TASK-AG-TG-002 touch disjoint files and have no code dependency on each other
— either can be implemented first, or both in parallel. They share only a reviewed pattern (the
same `git.execStream` precedent), not an ordering constraint; see each task's own "Depends on"
line for the exact relationship.
