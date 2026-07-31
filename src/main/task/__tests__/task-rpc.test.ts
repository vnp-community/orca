/**
 * Tests for task RPC methods (TDD-18) — T08
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 * Actual method names: task.create, task.get, task.update, task.delete,
 *   task.addEdge, task.grant, task.aiDecompose, task.aiApply, task.execute,
 *   task.resolvePermission, task.addComment
 *
 * RPC handler uses defineMethod() pattern — handler is at .handler (not .fn).
 * Grant method: task.grant (not task.grantAccess).
 * AI method: task.aiDecompose (not task.decomposeWithAI).
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
  labels: [],
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
    getSubtree: vi.fn().mockResolvedValue([]),
    recalculateProgress: vi.fn().mockResolvedValue(50),
    ...overrides,
  }
}

function makeGrantService(permission: string | null = 'manage') {
  return {
    resolvePermission: vi.fn().mockResolvedValue(permission),
    assertPermission: permission
      ? vi.fn().mockResolvedValue(undefined)
      : vi.fn().mockRejectedValue(Object.assign(new Error('TASK_ACCESS_DENIED'), { code: 'TASK_ACCESS_DENIED' })),
    grantPermission: vi.fn().mockResolvedValue('grant-id-001'),
    revokeGrant: vi.fn().mockResolvedValue(undefined),
    listGrants: vi.fn().mockResolvedValue([]),
  }
}

function makeAIPlanner() {
  return {
    decompose: vi.fn().mockResolvedValue([{ title: 'Sub A', type: 'subtask', id: 'sub-a' }]),
    applyDecomposition: vi.fn().mockResolvedValue([FAKE_TASK]),
  }
}

function makeExecutor() {
  return {
    executeTask: vi.fn().mockResolvedValue(undefined),
    buildPrompt: vi.fn().mockReturnValue('Prompt text'),
  }
}

// Find a method by name in the RpcMethod[] array
function findMethod(methods: ReturnType<typeof createTaskMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) throw new Error(`Method not found: ${name}`)
  return method
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('task RPC methods', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── task.create ────────────────────────────────────────────────────────────
  describe('task.create', () => {
    it('returns created task when valid params provided', async () => {
      const svc = makeTaskService()
      const methods = createTaskMethods(svc as any, makeGrantService() as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.create')
      const result = await method.handler({ title: 'Test task', type: 'task', priority: 'high', visibility: 'team', status: 'backlog' }, makeCtx('user-001'))
      expect(result.id).toBe('task-001')
    })

    it('delegates to taskService.create with all params', async () => {
      const svc = makeTaskService()
      const methods = createTaskMethods(svc as any, makeGrantService() as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.create')
      await method.handler({ title: 'Task', type: 'task', priority: 'medium', visibility: 'team', status: 'backlog' }, makeCtx('user-001'))
      expect(svc.create).toHaveBeenCalled()
    })
  })

  // ── task.get ───────────────────────────────────────────────────────────────
  describe('task.get', () => {
    it('returns task when user has view permission', async () => {
      const grantSvc = makeGrantService('view')
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.get')
      const result = await method.handler({ taskId: 'task-001' }, makeCtx('user-001'))
      expect(result.id).toBe('task-001')
    })

    it('throws TASK_PERMISSION_DENIED when user has no permission', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.get')
      await expect(method.handler({ taskId: 'task-001' }, makeCtx('user-nobody'))).rejects.toThrow('TASK_PERMISSION_DENIED')
    })
  })

  // ── task.execute ───────────────────────────────────────────────────────────
  describe('task.execute', () => {
    it('calls executor.executeTask with correct params', async () => {
      const executor = makeExecutor()
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('execute') as any, makeAIPlanner() as any, executor as any)
      const method = findMethod(methods, 'task.execute')
      await method.handler({ taskId: 'task-001', projectId: 'proj-001', worktreePath: '/repo' }, makeCtx('user-001'))
      expect(executor.executeTask).toHaveBeenCalledWith(expect.objectContaining({
        taskId: 'task-001',
        projectId: 'proj-001',
        userId: 'user-001',
      }))
    })

    it('returns { started: true }', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('execute') as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.execute')
      const result = await method.handler({ taskId: 'task-001', projectId: 'proj-001', worktreePath: '/repo' }, makeCtx('user-001'))
      expect(result).toEqual({ started: true })
    })
  })

  // ── task.aiDecompose ───────────────────────────────────────────────────────
  describe('task.aiDecompose', () => {
    it('user with edit perm receives proposals array', async () => {
      const planner = makeAIPlanner()
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('edit') as any, planner as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.aiDecompose')
      const result = await method.handler({ taskId: 'task-001', projectId: 'proj-001' }, makeCtx('user-001'))
      expect(Array.isArray(result)).toBe(true)
    })

    it('throws TASK_PERMISSION_DENIED when user has no edit permission', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.aiDecompose')
      await expect(method.handler({ taskId: 'task-001', projectId: 'proj-001' }, makeCtx('user-viewer'))).rejects.toThrow('TASK_PERMISSION_DENIED')
    })
  })

  // ── task.grant ──────────────────────────────────────────────────────────────
  describe('task.grant', () => {
    it('user with manage perm can grant permission', async () => {
      const grantSvc = makeGrantService('manage')
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.grant')
      const result = await method.handler({ taskId: 'task-001', scope: 'user', scopeId: 'user-002', permission: 'view' }, makeCtx('user-manager'))
      expect(grantSvc.grantPermission).toHaveBeenCalled()
      expect(result).toHaveProperty('grantId')
    })

    it('user without manage perm → TASK_ACCESS_DENIED', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService(null) as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.grant')
      await expect(method.handler({ taskId: 'task-001', scope: 'user', scopeId: 'u', permission: 'view' }, makeCtx('user-editor'))).rejects.toThrow()
    })
  })

  // ── task.addEdge ───────────────────────────────────────────────────────────
  describe('task.addEdge', () => {
    it('valid edge inserted — returns { added: true }', async () => {
      const methods = createTaskMethods(makeTaskService() as any, makeGrantService('edit') as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.addEdge')
      const result = await method.handler({ fromTaskId: 'task-001', toTaskId: 'task-002', edgeType: 'depends_on' }, makeCtx('u'))
      expect(result).toEqual({ added: true })
    })

    it('cycle detection → TASK_DAG_CYCLE thrown', async () => {
      const cycleService = makeTaskService({
        addEdge: vi.fn().mockRejectedValue(new Error('TASK_DAG_CYCLE: cycle')),
      })
      const methods = createTaskMethods(cycleService as any, makeGrantService('edit') as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.addEdge')
      await expect(method.handler({ fromTaskId: 'a', toTaskId: 'b', edgeType: 'depends_on' }, makeCtx('u'))).rejects.toThrow('TASK_DAG_CYCLE')
    })
  })

  // ── task.resolvePermission ─────────────────────────────────────────────────
  describe('task.resolvePermission', () => {
    it('returns { permission } for the user', async () => {
      const grantSvc = makeGrantService('edit')
      const methods = createTaskMethods(makeTaskService() as any, grantSvc as any, makeAIPlanner() as any, makeExecutor() as any)
      const method = findMethod(methods, 'task.resolvePermission')
      const result = await method.handler({ taskId: 'task-001' }, makeCtx('user-001'))
      expect(result).toHaveProperty('permission')
    })
  })
})
