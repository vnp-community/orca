# TASK-039: TaskAgentExecutor

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §5  
**Prerequisite:** TASK-015 (ProfileAwareAgentSpawner), TASK-037 (TaskGrantService)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/TaskAgentExecutor.ts`

Executes agent on a task, updates status, posts activity comment:

```typescript
export class TaskAgentExecutor {
  constructor(
    private readonly taskService: TaskService,
    private readonly agentSpawner: ProfileAwareAgentSpawner,
    private readonly grantService: TaskGrantService
  ) {}

  async executeTask(params: {
    taskId: string
    projectId: string
    userId: string
    worktreePath: string
    accountId?: string
  }): Promise<void>
  
  /**
   * Builds agent prompt from task context:
   * - task.promptTemplate (if set) with ${task.*} interpolation
   * - Or auto-generated from task.title + description + aiContext
   */
  buildPrompt(task: OrcaTask): string
}
```

**executeTask flow:**
1. Check grant: `grantService.resolvePermission(userId, taskId)` — need `execute` or `manage`
2. Get task → build prompt
3. Update status to `in_progress`
4. Call `agentSpawner.spawn({ projectId, userId, worktreePath, prompt, taskId })`
5. On success: update status to `review`, add activity comment
6. On error: update status to `blocked`, add error comment

## Acceptance Criteria

- [x] `TaskAgentExecutor` class export
- [x] Check permission before spawn (need execute/manage)
- [x] Status lifecycle: in_progress → review (success) or blocked (error)
- [x] Activity comment added on start, success, and error
- [x] `buildPrompt` resolves ${task.*} placeholders or auto-generates
- [x] Không TypeScript errors
