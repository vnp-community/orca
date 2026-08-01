# SOLUTION: Task Graph Domain — Fix tất cả Bugs

**Domain:** task-graph  
**TDD Reference:** TDD-18 (Task Graph), TDD-08 (Agent Orchestration)  
**Files cần thay đổi:** `src/main/task/TaskService.ts`, `src/main/workflow/StepExecutors.ts`, `src/relay/agent-rpc-dispatch.ts`  
**Tổng số bugs:** 3 (TG-002, BE-TG-001, BE-TG-002)

---

## Tổng quan phụ thuộc

```
BUG-BE-TG-001 (agent.exec method name mismatch) — phải fix trước BE-TG-002
    └── BUG-BE-TG-002 (ai.complete relay handler missing)

BUG-TG-002 (task executor missing dep check) — độc lập
```

**Thứ tự fix:** `BE-TG-001 → BE-TG-002 → TG-002`

---

## BUG-BE-TG-001 — Fix agent.exec method name mismatch

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `StepExecutors.executeAgent()` gọi `relay.call('agent.exec', ...)` nhưng relay dispatch không có handler cho `agent.exec`.

### Fix — Align method name hoặc thêm handler

Theo SOLUTION từ agent specs: relay dispatch `agent-rpc-dispatch.ts` cần case `'agent.exec'`.

```typescript
// src/main/workflow/StepExecutors.ts

// HIỆN TẠI (gọi agent.exec — không có handler):
const result = await relay.call('agent.exec', {
  stepId:      step.id,
  prompt:      step.config['prompt'],
  worktreePath: step.config['worktreePath'],
  trustPreset: step.config['trustPreset'] ?? 'default',
})

// FIX Option A — Dùng 'agent.spawn' thay vì 'agent.exec' (nếu relay có agent.spawn):
const result = await relay.call('agent.spawn', {
  model:       step.config['model'] ?? 'claude-opus-4-5',
  trustPreset: step.config['trustPreset'] ?? 'standard',
  cwd:         step.config['worktreePath'],
  taskId:      step.id,
  userId:      context.userId,
  projectId:   context.projectId,
  accountId:   step.config['accountId'],
})

// FIX Option B — Thêm 'agent.exec' handler trong relay (recommended — preserve API):
// src/relay/agent-rpc-dispatch.ts:
case 'agent.exec': {
  // agent.exec = non-interactive agent execution (wait for completion)
  // Delegate to agent-exec-handler (already exists via RelayDispatcher)
  const execHandler = getAgentExecHandler()
  const execResult = await execHandler.execNonInteractive({
    prompt:      rpc.params?.prompt as string,
    worktreePath: rpc.params?.worktreePath as string,
    trustPreset: rpc.params?.trustPreset as string ?? 'standard',
    stepId:      rpc.params?.stepId as string,
  })
  return makeOk(rpc.id, execResult)
}
```

---

## BUG-BE-TG-002 — Fix ai.complete relay handler missing

**Mức độ:** 🔴 CRITICAL  
**Root cause:** `TaskService.completeWithAI()` gọi `relay.call('ai.complete', ...)` nhưng handler không tồn tại.

### Fix — Thêm ai.complete handler

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm case:

case 'ai.complete': {
  // ai.complete = call AI provider API directly (non-interactive)
  // Params: { prompt, model, maxTokens, systemPrompt }
  const { prompt, model, maxTokens, systemPrompt, accountId } = rpc.params ?? {}

  if (!prompt || typeof prompt !== 'string') {
    return makeError(rpc.id, AgentErrorCode.InvalidParams, 'Missing prompt')
  }

  try {
    // Load credential
    const credStore = getCredentialStore(config)
    const apiKey    = await credStore.readDecrypted(accountId as string)
    if (!apiKey) {
      return makeError(rpc.id, AgentErrorCode.ServerError, 'No credential found for accountId')
    }

    // Route đến đúng provider
    const resolvedModel = (model as string) ?? 'claude-opus-4-5'
    const completion = await callAiProvider({
      model:        resolvedModel,
      apiKey,
      prompt:       prompt as string,
      systemPrompt: systemPrompt as string | undefined,
      maxTokens:    (maxTokens as number) ?? 4096,
    })

    return makeOk(rpc.id, {
      text:         completion.text,
      inputTokens:  completion.usage?.inputTokens,
      outputTokens: completion.usage?.outputTokens,
      model:        resolvedModel,
    })
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
  }
}

// Helper:
async function callAiProvider(params: {
  model:        string
  apiKey:       string
  prompt:       string
  systemPrompt?: string
  maxTokens:    number
}): Promise<{ text: string; usage?: { inputTokens: number; outputTokens: number } }> {
  if (params.model.startsWith('claude')) {
    return callAnthropic(params)
  } else if (params.model.startsWith('gpt') || params.model.startsWith('o1')) {
    return callOpenAI(params)
  } else if (params.model.startsWith('gemini')) {
    return callGoogle(params)
  }
  throw new Error(`Unknown model provider: ${params.model}`)
}
```

---

## BUG-TG-002 — Fix task executor thiếu dependency check

**Mức độ:** 🟠 HIGH  
**Root cause:** Task executor bắt đầu execute tasks mà không check xem dependencies (parent tasks) đã hoàn thành chưa.

### Fix — Thêm dependency check trước khi execute

```typescript
// src/main/task/TaskService.ts

export class TaskService {
  /**
   * Execute task chỉ khi tất cả dependencies đã hoàn thành.
   * FIX TG-002: Add dependency validation.
   */
  async executeTask(taskId: string, context: ExecutionContext): Promise<void> {
    const task = await this.repository.findById(taskId)
    if (!task) throw new Error(`Task not found: ${taskId}`)

    // FIX: Check dependencies TRƯỚC khi execute
    const unmetDeps = await this.getUnmetDependencies(task)
    if (unmetDeps.length > 0) {
      const depIds = unmetDeps.map(d => d.id).join(', ')
      throw new Error(`Task ${taskId} has unmet dependencies: ${depIds}`)
    }

    // Proceed with execution
    await this.repository.updateStatus(taskId, 'running')
    
    try {
      await this.runTaskSteps(task, context)
      await this.repository.updateStatus(taskId, 'completed')
    } catch (err) {
      await this.repository.updateStatus(taskId, 'failed', String(err))
      throw err
    }
  }

  /**
   * Get list of unmet dependencies (not yet completed).
   */
  private async getUnmetDependencies(task: Task): Promise<Task[]> {
    if (!task.dependencies || task.dependencies.length === 0) return []

    const depTasks = await Promise.all(
      task.dependencies.map(depId => this.repository.findById(depId))
    )

    return depTasks.filter((dep): dep is Task =>
      dep !== null && dep.status !== 'completed'
    )
  }

  /**
   * Process task queue — execute in dependency order (topological sort).
   */
  async processQueue(graphId: string): Promise<void> {
    const tasks = await this.repository.listByGraph(graphId)
    const sorted = this.topologicalSort(tasks)

    for (const task of sorted) {
      if (task.status === 'pending') {
        await this.executeTask(task.id, this.createContext(task)).catch(err => {
          // Log but continue with other tasks
          this.log.error(`[TaskGraph] Task ${task.id} failed:`, err)
        })
      }
    }
  }

  /**
   * Topological sort để đảm bảo execute đúng thứ tự.
   */
  private topologicalSort(tasks: Task[]): Task[] {
    const graph = new Map(tasks.map(t => [t.id, t]))
    const visited = new Set<string>()
    const sorted: Task[] = []

    function visit(taskId: string): void {
      if (visited.has(taskId)) return
      visited.add(taskId)

      const task = graph.get(taskId)
      if (!task) return

      for (const depId of task.dependencies ?? []) {
        visit(depId)
      }

      sorted.push(task)
    }

    for (const task of tasks) {
      visit(task.id)
    }

    return sorted
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/relay/agent-rpc-dispatch.ts` | Add `case 'agent.exec'` | BE-TG-001 |
| `src/relay/agent-rpc-dispatch.ts` | Add `case 'ai.complete'` | BE-TG-002 |
| `src/relay/ai-provider-caller.ts` | NEW — Anthropic/OpenAI/Google API callers | BE-TG-002 |
| `src/main/task/TaskService.ts` | Add dependency check + topological sort | TG-002 |

---

## Verification Plan

```bash
# Test BE-TG-001:
# 1. Send agent.exec → verify handled (not 'method not found')
# 2. Send agent.spawn → verify still works

# Test BE-TG-002:
# 1. Send ai.complete { prompt, model:'claude-opus-4-5' } → verify completion returned
# 2. Invalid model → verify error response

# Test TG-002:
# 1. Task with deps not completed → verify blocked with error
# 2. Complete deps → trigger task → verify runs
# 3. Circular dep → verify topological sort handles gracefully

pnpm vitest run src/main/task/__tests__/
pnpm vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
```
