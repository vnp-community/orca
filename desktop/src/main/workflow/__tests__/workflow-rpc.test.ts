/**
 * Tests for workflow RPC handlers (TDD-17) — T14
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 *
 * Actual API:
 *   createWorkflowMethods(orchestrator: WorkflowOrchestrator, templateResolver: TemplateResolver)
 *
 * Actual method names: workflow.execute, workflow.getExecution, workflow.listExecutions,
 *   workflow.cancel, workflow.template.create, workflow.template.list, workflow.template.resolve
 *
 * NOTE:
 * - 'workflow.resumeAll' does NOT exist in implementation
 * - orchestrator.execute() not orchestrator.start()
 * - workflow.execute returns { executionId, status }
 * - workflow.cancel checks execution.triggeredBy === userId (WORKFLOW_CANCEL_DENIED if mismatch)
 * - workflow.template.create uses templateResolver.create() returning template ID (not object)
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createWorkflowMethods } from '../workflow-rpc-handler'
import type { RpcContext } from '../../runtime/rpc/core'

// ── Helpers ────────────────────────────────────────────────────────────────────

const FAKE_EXEC_ID = 'exec-001'

const FAKE_EXECUTION = {
  id: FAKE_EXEC_ID,
  templateId: 'tmpl-001',
  projectId: 'proj-001',
  triggeredBy: 'user-001',
  status: 'running',
  startedAt: new Date(),
  steps: [],
}

const FAKE_TEMPLATE = {
  id: 'tmpl-001',
  name: 'CI Pipeline',
  steps: [],
}

function makeCtx(userId: string, role = 'developer'): RpcContext {
  return { userId, user: { id: userId, role } } as unknown as RpcContext
}

function makeOrchestrator(overrides = {}) {
  return {
    execute: vi.fn().mockResolvedValue({ id: FAKE_EXEC_ID, status: 'running' }),
    cancel: vi.fn().mockResolvedValue(undefined),
    getExecution: vi.fn().mockResolvedValue(FAKE_EXECUTION),
    listExecutions: vi.fn().mockResolvedValue([FAKE_EXECUTION]),
    resumeRunningExecutions: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function makeTemplateResolver(overrides = {}) {
  return {
    resolve: vi.fn().mockResolvedValue(FAKE_TEMPLATE),
    create: vi.fn().mockResolvedValue('tmpl-new-001'),  // returns ID string
    list: vi.fn().mockResolvedValue([FAKE_TEMPLATE]),
    get: vi.fn().mockResolvedValue(FAKE_TEMPLATE),
    update: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function findHandler(methods: ReturnType<typeof createWorkflowMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) {throw new Error(`Method not found: ${name}`)}
  return method.handler
}

// ── Tests ──────────────────────────────────────────────────────────────────────

describe('workflow RPC handlers', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── workflow.execute ────────────────────────────────────────────────────────
  describe('workflow.execute', () => {
    it('authenticated user can start workflow — returns executionId', async () => {
      const orch = makeOrchestrator()
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      const result = await handler(
        { definition: { name: 'test', steps: [] }, projectId: 'proj-001', inputs: {} },
        makeCtx('user-001')
      )
      expect(result.executionId).toBe(FAKE_EXEC_ID)
      expect(orch.execute).toHaveBeenCalled()
    })

    it('returns { executionId, status } object', async () => {
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      const result = await handler(
        { definition: { name: 'test', steps: [] }, projectId: 'proj-001', inputs: {} },
        makeCtx('user-001')
      )
      expect(result).toHaveProperty('executionId')
      expect(result).toHaveProperty('status')
    })

    it('propagates TEMPLATE_NOT_FOUND from orchestrator', async () => {
      const orch = makeOrchestrator({
        execute: vi.fn().mockRejectedValue(new Error('TEMPLATE_NOT_FOUND')),
      })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      await expect(
        handler({ definition: { name: 'bad', steps: [] }, projectId: 'proj-001', inputs: {} }, makeCtx('user-001'))
      ).rejects.toThrow('TEMPLATE_NOT_FOUND')
    })

    it('propagates WORKFLOW_CYCLE from orchestrator', async () => {
      const orch = makeOrchestrator({
        execute: vi.fn().mockRejectedValue(new Error('WORKFLOW_CYCLE')),
      })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      await expect(
        handler({ definition: { name: 'cyclic', steps: [] }, projectId: 'proj-001', inputs: {} }, makeCtx('user-001'))
      ).rejects.toThrow('WORKFLOW_CYCLE')
    })
  })

  // ── workflow.cancel ─────────────────────────────────────────────────────────
  describe('workflow.cancel', () => {
    it('triggeredBy user can cancel their own execution', async () => {
      const orch = makeOrchestrator()
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.cancel')
      const result = await handler({ executionId: FAKE_EXEC_ID }, makeCtx('user-001'))
      expect(orch.cancel).toHaveBeenCalledWith(FAKE_EXEC_ID)
      expect(result).toHaveProperty('cancelled', true)
    })

    it('different user cannot cancel — WORKFLOW_CANCEL_DENIED', async () => {
      // FAKE_EXECUTION.triggeredBy = 'user-001', caller is 'user-other'
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.cancel')
      await expect(
        handler({ executionId: FAKE_EXEC_ID }, makeCtx('user-other'))
      ).rejects.toThrow('WORKFLOW_CANCEL_DENIED')
    })

    it('unknown executionId propagates EXECUTION_NOT_FOUND', async () => {
      const orch = makeOrchestrator({
        getExecution: vi.fn().mockResolvedValue(null),
      })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.cancel')
      await expect(
        handler({ executionId: 'unknown' }, makeCtx('user-001'))
      ).rejects.toThrow('EXECUTION_NOT_FOUND')
    })
  })

  // ── workflow.getExecution ───────────────────────────────────────────────────
  describe('workflow.getExecution', () => {
    it('returns execution with step statuses', async () => {
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.getExecution')
      const result = await handler({ executionId: FAKE_EXEC_ID }, makeCtx('user-001'))
      expect(result.id).toBe(FAKE_EXEC_ID)
    })

    it('throws EXECUTION_NOT_FOUND when execution is null', async () => {
      const orch = makeOrchestrator({ getExecution: vi.fn().mockResolvedValue(null) })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.getExecution')
      await expect(
        handler({ executionId: 'unknown' }, makeCtx('user-001'))
      ).rejects.toThrow('EXECUTION_NOT_FOUND')
    })
  })

  // ── workflow.listExecutions ─────────────────────────────────────────────────
  describe('workflow.listExecutions', () => {
    it('returns array of executions', async () => {
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.listExecutions')
      const result = await handler({ projectId: 'proj-001' }, makeCtx('user-001'))
      expect(Array.isArray(result)).toBe(true)
    })

    it('passes status filter to orchestrator.listExecutions', async () => {
      const orch = makeOrchestrator({ listExecutions: vi.fn().mockResolvedValue([]) })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.listExecutions')
      await handler({ projectId: 'proj-001', status: 'running' }, makeCtx('user-001'))
      expect(orch.listExecutions).toHaveBeenCalledWith(expect.objectContaining({ status: 'running' }))
    })
  })

  // ── workflow.template.create ────────────────────────────────────────────────
  describe('workflow.template.create', () => {
    it('creates template — returns { templateId }', async () => {
      const resolver = makeTemplateResolver()
      const methods = createWorkflowMethods(makeOrchestrator() as any, resolver as any)
      const handler = findHandler(methods, 'workflow.template.create')
      const result = await handler(
        { name: 'Deploy Pipeline', definition: { name: 'Deploy', steps: [] }, scope: 'project' },
        makeCtx('admin-001', 'admin')
      )
      expect(result).toHaveProperty('templateId')
    })

    it('delegates to templateResolver.create', async () => {
      const resolver = makeTemplateResolver()
      const methods = createWorkflowMethods(makeOrchestrator() as any, resolver as any)
      const handler = findHandler(methods, 'workflow.template.create')
      await handler(
        { name: 'My Template', definition: { name: 'My', steps: [] }, scope: 'project' },
        makeCtx('user-001')
      )
      expect(resolver.create).toHaveBeenCalled()
    })
  })

  // ── workflow.template.list ──────────────────────────────────────────────────
  describe('workflow.template.list', () => {
    it('returns list of templates for given scope', async () => {
      const resolver = makeTemplateResolver()
      const methods = createWorkflowMethods(makeOrchestrator() as any, resolver as any)
      const handler = findHandler(methods, 'workflow.template.list')
      const result = await handler({ scope: 'project' }, makeCtx('user-001'))
      expect(Array.isArray(result)).toBe(true)
    })
  })

  // ── workflow.template.resolve ───────────────────────────────────────────────
  describe('workflow.template.resolve', () => {
    it('returns resolved template for given templateId', async () => {
      const resolver = makeTemplateResolver()
      const methods = createWorkflowMethods(makeOrchestrator() as any, resolver as any)
      const handler = findHandler(methods, 'workflow.template.resolve')
      const result = await handler({ templateId: 'tmpl-001' }, makeCtx('user-001'))
      expect(result.id).toBe('tmpl-001')
    })
  })
})
