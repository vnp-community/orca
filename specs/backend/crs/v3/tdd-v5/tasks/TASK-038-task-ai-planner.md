# TASK-038: TaskAIPlanner

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §5  
**Prerequisite:** TASK-035, TASK-022 (ProviderResolver)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/TaskAIPlanner.ts`

AI-powered task decomposition:

```typescript
export class TaskAIPlanner {
  constructor(
    private readonly taskService: TaskService,
    private readonly providerService: AIProviderService,
    private readonly router: ProjectServerRouter
  ) {}

  /**
   * Decompose a task into subtasks using AI.
   * Calls relay ai.complete with structured prompt.
   * Returns list of proposed subtasks (NOT yet persisted).
   */
  async decompose(taskId: string, projectId: string, userId: string): Promise<OrcaTask[]>

  /**
   * Apply AI-generated subtasks to task (create as children).
   */
  async applyDecomposition(taskId: string, subtasks: Array<Partial<OrcaTask>>): Promise<OrcaTask[]>

  /**
   * Generate prompt template for a task based on its context.
   */
  async generatePromptTemplate(taskId: string, userId: string): Promise<string>
}
```

**decompose() prompt:**
```
You are a software project manager. Decompose the following task into 3-7 concrete subtasks.
Return JSON array: [{ "title": string, "type": "subtask"|"task", "estimatedHours": number }]
Task: {task.title}
Description: {task.description}
```

## Acceptance Criteria

- [x] `TaskAIPlanner` class export
- [x] `decompose()` calls relay ai.complete with structured JSON prompt
- [x] `applyDecomposition()` creates tasks as children via TaskService
- [x] `generatePromptTemplate()` resolves ${task.*} placeholders
- [x] Returns structured `OrcaTask[]`
- [x] Không TypeScript errors
