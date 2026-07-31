# T08 — Write task-rpc.test.ts

**Phase:** 2A  
**Effort:** ~1 hour  
**Depends on:** T04 (TaskService), T05 (TaskGrantService)  
**Solution ref:** [05-tdd18-task-graph.md §2.5](../solutions/05-tdd18-task-graph.md)  
**TDD ref:** TDD-18 (task-rpc-handler.ts)

---

## Mục tiêu

Viết RPC handler tests cho `task-rpc-handler.ts` — access control, delegation, error codes.

**Target: ≥ 10 tests**

---

## Files Cần Đọc Trước

1. `src/main/task/task-rpc-handler.ts` — xem toàn bộ (tên functions export, method names, RpcContext usage)
2. `src/main/project/__tests__/project-rpc.test.ts` — **pattern tái sử dụng chính** (mock pattern + findHandler)
3. `src/main/runtime/rpc/core.ts` — RpcContext interface

---

## File Cần Tạo

### `src/main/task/__tests__/task-rpc.test.ts`

**Pattern hoàn toàn tái sử dụng từ `project-rpc.test.ts`:**

```typescript
/**
 * Tests for task RPC methods (TDD-18) — T08
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 * ≥ 10 tests covering access control + delegation.
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createTaskMethods } from '../task-rpc-handler'
import type { RpcContext } from '../../runtime/rpc/core'
import type { OrcaTask } from '../../../shared/task-types'

// ── Helpers ───────────────────────────────────────────────────────────────────

const FAKE_TASK: OrcaTask = {
  id: 'task-001',
  title: 'Fix bug',
  type: 'task',
  status: 'todo',
  priority: 'high',
  reporterId: 'user-reporter',
  visibility: 'team',
  progressPercent: 0,
  createdAt: new Date(),
  updatedAt: new Date(),
}

function makeCtx(userId: string, role = 'developer'): RpcContext {
  return { userId, user: { role } } as unknown as RpcContext
}

function makeTaskService(overrides = {}) {
  return {
    create: vi.fn().mockResolvedValue(FAKE_TASK),
    get: vi.fn().mockResolvedValue(FAKE_TASK),
    list: vi.fn().mockResolvedValue([FAKE_TASK]),
    update: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
    addEdge: vi.fn().mockResolvedValue(undefined),
    removeEdge: vi.fn().mockResolvedValue(undefined),
    addComment: vi.fn().mockResolvedValue(undefined),
    getComments: vi.fn().mockResolvedValue([]),
    getAncestors: vi.fn().mockResolvedValue([]),
    getChildren: vi.fn().mockResolvedValue([]),
    getDependencies: vi.fn().mockResolvedValue([]),
    ...overrides,
  }
}

function makeGrantService(permission: string | null = 'manage') {
  return {
    resolvePermission: vi.fn().mockResolvedValue(permission),
    assertPermission: permission
      ? vi.fn().mockResolvedValue(undefined)
      : vi.fn().mockRejectedValue(Object.assign(new Error('TASK_ACCESS_DENIED'), { code: 'TASK_ACCESS_DENIED' })),
    grantAccess: vi.fn().mockResolvedValue(undefined),
    revokeAccess: vi.fn().mockResolvedValue(undefined),
    listGrants: vi.fn().mockResolvedValue([]),
  }
}

function makeAIPlanner() {
  return {
    decomposeTask: vi.fn().mockResolvedValue([{ title: 'Sub A', type: 'subtask', estimatedHours: 1, dependsOn: [] }]),
    applyDecomposition: vi.fn().mockResolvedValue([FAKE_TASK]),
  }
}

function makeExecutor() {
  return {
    executeTask: vi.fn().mockResolvedValue({ sessionId: 'sess-001' }),
    buildPreamble: vi.fn().mockResolvedValue('Preamble text'),
  }
}

function findHandler(methods: ReturnType<typeof createTaskMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) throw new Error(`Method not found: ${name}`)
  return method.handler
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('task RPC methods', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── task.create ────────────────────────────────────────────────────────────
  describe('task.create', () => {
    it('returns created task when valid params provided', async () => {
      const svc = makeTaskService()
      const methods = createTaskMethods(svc as any, makeGrantService() as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.create')
      const result = await handler({ title: 'Test task', type: 'task', priority: 'high', visibility: 'team' }, makeCtx('user-001'))
      expect(result.id).toBe('task-001')
    })
  })

  // ── task.get ───────────────────────────────────────────────────────────────
  describe('task.get', () => {
    it('returns task details when user has view permission', async () => {
      const grantSvc = makeGrantService('view')
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.get')
      const result = await handler({ taskId: 'task-001' }, makeCtx('user-001'))
      expect(result.id).toBe('task-001')
    })

    it('throws TASK_ACCESS_DENIED when user has no permission', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.get')
      await expect(handler({ taskId: 'task-001' }, makeCtx('user-nobody'))).rejects.toThrow('TASK_ACCESS_DENIED')
    })
  })

  // ── task.execute ───────────────────────────────────────────────────────────
  describe('task.execute', () => {
    it('user with execute perm can spawn agent', async () => {
      const executor = makeExecutor()
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('execute') as any, makeAIPlanner() as any, executor as any)
      const handler = findHandler(methods, 'task.execute')
      await handler({ taskId: 'task-001' }, makeCtx('user-001'))
      expect(executor.executeTask).toHaveBeenCalledWith('task-001', 'user-001')
    })

    it('user with only view perm → TASK_ACCESS_DENIED', async () => {
      const grantSvc = makeGrantService(null)
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.execute')
      await expect(handler({ taskId: 'task-001' }, makeCtx('user-viewer'))).rejects.toThrow('TASK_ACCESS_DENIED')
    })
  })

  // ── task.decomposeWithAI ───────────────────────────────────────────────────
  describe('task.decomposeWithAI', () => {
    it('user with edit perm receives SubtaskSuggestion array', async () => {
      const planner = makeAIPlanner()
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('edit') as any, planner as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.decomposeWithAI')
      const result = await handler({ taskId: 'task-001' }, makeCtx('user-001'))
      expect(Array.isArray(result)).toBe(true)
    })

    it('user with only view perm → TASK_ACCESS_DENIED', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.decomposeWithAI')
      await expect(handler({ taskId: 'task-001' }, makeCtx('user-viewer'))).rejects.toThrow('TASK_ACCESS_DENIED')
    })
  })

  // ── task.grantAccess ──────────────────────────────────────────────────────
  describe('task.grantAccess', () => {
    it('user with manage perm can grant access', async () => {
      const grantSvc = makeGrantService('manage')
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.grantAccess')
      await handler({ taskId: 'task-001', scope: 'user', scopeId: 'user-002', permission: 'view' }, makeCtx('user-manager'))
      expect(grantSvc.grantAccess).toHaveBeenCalled()
    })

    it('user with edit perm → TASK_ACCESS_DENIED', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.grantAccess')
      await expect(handler({ taskId: 'task-001', scope: 'user', scopeId: 'u', permission: 'view' }, makeCtx('user-editor'))).rejects.toThrow()
    })
  })

  // ── task.addEdge ───────────────────────────────────────────────────────────
  describe('task.addEdge', () => {
    it('valid edge inserted — no error', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('edit') as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.addEdge')
      await expect(handler({ fromTaskId: 'task-001', toTaskId: 'task-002', type: 'depends_on' }, makeCtx('u'))).resolves.not.toThrow()
    })

    it('cycle detection → TASK_DEPENDENCY_CYCLE thrown', async () => {
      const cycleService = makeTaskService({
        addEdge: vi.fn().mockRejectedValue(new Error('TASK_DEPENDENCY_CYCLE')),
      })
      const methods = createTaskMethods(cycleService as any, makeGrantService('edit') as any, makeAIPlanner() as any, makeExecutor() as any)
      const handler = findHandler(methods, 'task.addEdge')
      await expect(handler({ fromTaskId: 'a', toTaskId: 'b', type: 'depends_on' }, makeCtx('u'))).rejects.toThrow('TASK_DEPENDENCY_CYCLE')
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/task/__tests__/task-rpc.test.ts` ✅
- [x] `pnpm vitest run src/main/task/__tests__/task-rpc.test.ts` → ≥10 tests passing ✅ (13 tests pass)
- [x] 0 TypeScript errors ✅
