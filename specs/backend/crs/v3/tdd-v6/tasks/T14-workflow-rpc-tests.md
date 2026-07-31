# T14 — Write workflow-rpc.test.ts

**Phase:** 2C  
**Effort:** ~45 min  
**Depends on:** — (independent)  
**Solution ref:** [04-tdd17-workflow-orchestration.md §2.1](../solutions/04-tdd17-workflow-orchestration.md)  
**TDD ref:** TDD-17 (workflow-rpc-handler.ts)

---

## Mục tiêu

Viết RPC handler tests cho `workflow-rpc-handler.ts` — execute, cancel, access control.

**Target: ≥ 12 tests**

---

## Files Cần Đọc Trước

1. `src/main/workflow/workflow-rpc-handler.ts` — đọc toàn bộ (method names, createWorkflowMethods export)
2. `src/main/project/__tests__/project-rpc.test.ts` — **pattern tái sử dụng** (mock + findHandler)
3. `src/main/workflow/WorkflowTypes.ts` — WorkflowDefinition, StepType types

---

## File Cần Tạo

### `src/main/workflow/__tests__/workflow-rpc.test.ts`

```typescript
/**
 * Tests for workflow RPC handlers (TDD-17) — T14
 *
 * Mock-based tests following project-rpc.test.ts pattern.
 * ≥ 12 tests for execute/cancel/resume/template methods.
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createWorkflowMethods } from '../workflow-rpc-handler'
import type { RpcContext } from '../../runtime/rpc/core'

// ── Helpers ────────────────────────────────────────────────────────────────────

const FAKE_EXEC_ID = 'exec-001'
const FAKE_TEMPLATE = {
  id: 'tmpl-001',
  name: 'CI Pipeline',
  steps: [
    { id: 'step-1', name: 'Build', type: 'shell' as const, config: { command: 'npm run build' }, dependsOn: [] },
    { id: 'step-2', name: 'Test', type: 'shell' as const, config: { command: 'npm test' }, dependsOn: ['step-1'] },
  ],
}

const FAKE_EXECUTION = {
  id: FAKE_EXEC_ID,
  templateId: 'tmpl-001',
  projectId: 'proj-001',
  triggeredBy: 'user-001',
  status: 'running',
  startedAt: new Date(),
  steps: [],
}

function makeCtx(userId: string, role = 'developer'): RpcContext {
  return { userId, user: { id: userId, role } } as unknown as RpcContext
}

function makeOrchestrator(overrides = {}) {
  return {
    start: vi.fn().mockResolvedValue(FAKE_EXEC_ID),
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
    create: vi.fn().mockResolvedValue(FAKE_TEMPLATE),
    list: vi.fn().mockResolvedValue([FAKE_TEMPLATE]),
    get: vi.fn().mockResolvedValue(FAKE_TEMPLATE),
    update: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function findHandler(methods: ReturnType<typeof createWorkflowMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) throw new Error(`Method not found: ${name}`)
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
        { templateId: 'tmpl-001', projectId: 'proj-001', inputs: {} },
        makeCtx('user-001')
      )
      expect(result.executionId).toBe(FAKE_EXEC_ID)
      expect(orch.start).toHaveBeenCalled()
    })

    it('invalid templateId returns TEMPLATE_NOT_FOUND', async () => {
      const orch = makeOrchestrator({
        start: vi.fn().mockRejectedValue(new Error('TEMPLATE_NOT_FOUND')),
      })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      await expect(
        handler({ templateId: 'nonexistent', projectId: 'proj-001', inputs: {} }, makeCtx('user-001'))
      ).rejects.toThrow('TEMPLATE_NOT_FOUND')
    })

    it('cycle in definition returns WORKFLOW_CYCLE', async () => {
      const orch = makeOrchestrator({
        start: vi.fn().mockRejectedValue(new Error('WORKFLOW_CYCLE')),
      })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.execute')
      await expect(
        handler({ templateId: 'cyclic-tmpl', projectId: 'proj-001', inputs: {} }, makeCtx('user-001'))
      ).rejects.toThrow('WORKFLOW_CYCLE')
    })
  })

  // ── workflow.cancel ─────────────────────────────────────────────────────────
  describe('workflow.cancel', () => {
    it('triggeredBy user can cancel their own execution', async () => {
      const orch = makeOrchestrator({ cancel: vi.fn().mockResolvedValue(undefined) })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.cancel')
      await handler({ executionId: FAKE_EXEC_ID }, makeCtx('user-001'))
      expect(orch.cancel).toHaveBeenCalledWith(FAKE_EXEC_ID, 'user-001')
    })

    it('unknown executionId returns EXECUTION_NOT_FOUND', async () => {
      const orch = makeOrchestrator({
        cancel: vi.fn().mockRejectedValue(new Error('EXECUTION_NOT_FOUND')),
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
    it('returns execution with step statuses for project member', async () => {
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.getExecution')
      const result = await handler({ executionId: FAKE_EXEC_ID }, makeCtx('user-001'))
      expect(result.id).toBe(FAKE_EXEC_ID)
    })

    it('unknown executionId returns 404/EXECUTION_NOT_FOUND', async () => {
      const orch = makeOrchestrator({ getExecution: vi.fn().mockResolvedValue(null) })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.getExecution')
      await expect(
        handler({ executionId: 'unknown' }, makeCtx('user-001'))
      ).rejects.toThrow()
    })
  })

  // ── workflow.listExecutions ─────────────────────────────────────────────────
  describe('workflow.listExecutions', () => {
    it('returns executions filtered by projectId', async () => {
      const orch = makeOrchestrator()
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.listExecutions')
      const result = await handler({ projectId: 'proj-001' }, makeCtx('user-001'))
      expect(Array.isArray(result)).toBe(true)
    })

    it('respects status filter (running/completed/failed)', async () => {
      const orch = makeOrchestrator({ listExecutions: vi.fn().mockResolvedValue([]) })
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.listExecutions')
      await handler({ projectId: 'proj-001', status: 'running' }, makeCtx('user-001'))
      expect(orch.listExecutions).toHaveBeenCalledWith(expect.objectContaining({ status: 'running' }))
    })
  })

  // ── workflow.template.create ────────────────────────────────────────────────
  describe('workflow.template.create', () => {
    it('admin creates template — returns template with id', async () => {
      const resolver = makeTemplateResolver()
      const methods = createWorkflowMethods(makeOrchestrator() as any, resolver as any)
      const handler = findHandler(methods, 'workflow.template.create')
      const result = await handler(
        { name: 'Deploy Pipeline', steps: FAKE_TEMPLATE.steps, projectId: 'proj-001' },
        makeCtx('admin-001', 'admin')
      )
      expect(result.id).toBeDefined()
    })
  })

  // ── workflow.resumeAll ──────────────────────────────────────────────────────
  describe('workflow.resumeAll', () => {
    it('admin triggers resume of all interrupted executions', async () => {
      const orch = makeOrchestrator()
      const methods = createWorkflowMethods(orch as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.resumeAll')
      await handler({}, makeCtx('admin-001', 'admin'))
      expect(orch.resumeRunningExecutions).toHaveBeenCalled()
    })

    it('non-admin receives WORKFLOW_UNAUTHORIZED', async () => {
      const methods = createWorkflowMethods(makeOrchestrator() as any, makeTemplateResolver() as any)
      const handler = findHandler(methods, 'workflow.resumeAll')
      await expect(handler({}, makeCtx('user-001', 'developer'))).rejects.toThrow()
    })
  })
})
```

---

## Acceptance Criteria

- [x] File tạo tại `src/main/workflow/__tests__/workflow-rpc.test.ts` ✅
- [x] `pnpm vitest run src/main/workflow/__tests__/workflow-rpc.test.ts` → ≥12 tests passing ✅ (15 tests pass)
- [x] 0 TypeScript errors ✅
