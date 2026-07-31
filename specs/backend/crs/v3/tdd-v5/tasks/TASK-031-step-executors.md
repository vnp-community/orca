# TASK-031: StepExecutors

**Phase:** 5 — Workflow Orchestration  
**Solution ref:** [SOL-V5-004](../solutions/SOL-V5-004-workflow-orchestration.md) §7  
**Prerequisite:** TASK-014 (ProjectServerRouter)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/workflow/StepExecutors.ts`

Routes step execution to correct dev server via relay:

```typescript
export class StepExecutors {
  constructor(private readonly router: ProjectServerRouter) {}

  async execute(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
    // Race: actual execution vs timeout
    return Promise.race([
      this.executeByType(step, signal),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error('STEP_TIMEOUT')), step.timeout ?? 30 * 60_000)
      )
    ])
  }

  private async executeByType(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    if (step.config.type === 'agent') { /* relay.call('agent.exec', ...) */ }
    if (step.config.type === 'shell') { /* relay.call('git.exec' or shell) */ }
    if (step.config.type === 'webhook') { /* fetch() */ }
    return { exitCode: 0 }
  }
}
```

## Acceptance Criteria

- [x] `StepExecutors` class export
- [x] `execute()` respects `signal.aborted`
- [x] Timeout applied via `Promise.race`
- [x] `webhook` type uses `fetch()` with signal
- [x] Không TypeScript errors
