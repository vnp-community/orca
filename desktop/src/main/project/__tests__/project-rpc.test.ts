/**
 * Tests for Project RPC Methods (TDD-15) — TASK-019
 *
 * Uses mocks for ProjectService and ProfileAwareAgentSpawner.
 * ≥ 5 tests covering key access control and delegation.
 *
 * @module main/project/__tests__/project-rpc.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { createProjectMethods } from '../project-rpc-handler'
import type { ProjectService } from '../ProjectService'
import type { ProfileAwareAgentSpawner } from '../ProfileAwareAgentSpawner'
import type { RpcContext } from '../../runtime/rpc/core'
import type { OrcaProject, ProjectMember } from '../../../shared/project-types'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

// ── helpers ────────────────────────────────────────────────────────────────

const FAKE_PROJECT: OrcaProject = {
  id: 'proj-1',
  name: 'Test Project',
  devServerId: 'srv-1',
  repoPath: '/repo',
  defaultBranch: 'main',
  visibility: 'team',
  createdBy: 'u-1',
  createdAt: new Date(),
  updatedAt: new Date(),
}

const OWNER_MEMBER: ProjectMember = {
  projectId: 'proj-1', userId: 'u-1', role: 'owner', addedAt: new Date()
}

const VIEWER_MEMBER: ProjectMember = {
  projectId: 'proj-1', userId: 'u-viewer', role: 'viewer', addedAt: new Date()
}

function makeCtx(userId: string): RpcContext {
  return { userId } as RpcContext
}

function makeService(overrides: Partial<ProjectService> = {}): ProjectService {
  return {
    list: vi.fn().mockResolvedValue([FAKE_PROJECT]),
    get: vi.fn().mockResolvedValue(FAKE_PROJECT),
    create: vi.fn().mockResolvedValue(FAKE_PROJECT),
    update: vi.fn().mockResolvedValue(undefined),
    delete: vi.fn().mockResolvedValue(undefined),
    addMember: vi.fn().mockResolvedValue(undefined),
    removeMember: vi.fn().mockResolvedValue(undefined),
    updateMemberRole: vi.fn().mockResolvedValue(undefined),
    getMembers: vi.fn().mockResolvedValue([OWNER_MEMBER]),
    getMember: vi.fn().mockResolvedValue(OWNER_MEMBER),
    assertAccess: vi.fn().mockResolvedValue(OWNER_MEMBER),
    ...overrides,
  } as unknown as ProjectService
}

function makeSpawner(): ProfileAwareAgentSpawner {
  return {
    spawn: vi.fn().mockResolvedValue({ sessionId: 'agent-1' }),
  } as unknown as ProfileAwareAgentSpawner
}

// FIX BUG-BE-HLD-002: createProjectMethods now requires getUserRole. Default stub
// returns null (non-admin) so existing owner/viewer-role assertions keep their
// original semantics — tests that need admin-override behavior pass their own stub.
const stubGetUserRole = async (_userId: string): Promise<'developer' | 'lead' | 'admin' | null> => null

/** Find a method handler by name from the factory output */
function findHandler(methods: ReturnType<typeof createProjectMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) {throw new Error(`Method not found: ${name}`)}
  return method.handler
}

describe('project RPC methods', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── 1. project.list → returns user projects ───────────────────────────────

  it('project.list: returns user projects via service.list', async () => {
    const service = makeService()
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.list')
    const result = await handler(undefined, makeCtx('u-1'))
    expect(service.list).toHaveBeenCalledWith('u-1')
    expect(result).toEqual([FAKE_PROJECT])
  })

  // ── 2. project.create → delegates to service, returns new project ─────────

  it('project.create: delegates to service with createdBy from userId', async () => {
    const service = makeService()
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.create')
    const params = {
      name: 'New Proj',
      devServerId: 'srv-1',
      repoPath: '/repo',
    }
    const result = await handler(params, makeCtx('u-1'))
    expect(service.create).toHaveBeenCalledWith(expect.objectContaining({
      name: 'New Proj',
      createdBy: 'u-1',
    }))
    expect(result).toBe(FAKE_PROJECT)
  })

  // ── 3. project.addMember: non-owner → throws FORBIDDEN ───────────────────

  it('project.addMember: viewer role → throws FORBIDDEN', async () => {
    const service = makeService({
      assertAccess: vi.fn().mockResolvedValue(VIEWER_MEMBER),
    })
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.addMember')
    await expect(
      handler({ projectId: 'proj-1', userId: 'u-2', role: 'member' }, makeCtx('u-viewer'))
    ).rejects.toThrow('FORBIDDEN')
  })

  // ── 4. project.agentSpawn → calls spawner.spawn ───────────────────────────

  it('project.agentSpawn: calls agentSpawner.spawn with userId and projectId', async () => {
    const service = makeService()
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, stubGetUserRole, spawner)
    const handler = findHandler(methods, 'project.agentSpawn')
    const result = await handler({ projectId: 'proj-1', command: 'echo hi' }, makeCtx('u-1'))
    expect(spawner.spawn).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: 'proj-1', userId: 'u-1', command: 'echo hi' })
    )
    expect((result as { sessionId: string }).sessionId).toBe('agent-1')
  })

  // ── 5. project.delete: non-owner → FORBIDDEN ─────────────────────────────

  it('project.delete: viewer → throws FORBIDDEN', async () => {
    const service = makeService({
      assertAccess: vi.fn().mockResolvedValue(VIEWER_MEMBER),
    })
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.delete')
    await expect(
      handler({ projectId: 'proj-1' }, makeCtx('u-viewer'))
    ).rejects.toThrow('FORBIDDEN')
  })

  // ── 6. project.list: throws UNAUTHENTICATED when no userId ───────────────

  it('project.list: throws UNAUTHENTICATED when ctx has no userId', async () => {
    const service = makeService()
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.list')
    await expect(handler(undefined, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
  })

  // ── 7. project.getMembers: delegates to service.getMembers ───────────────

  it('project.getMembers: returns members for valid member', async () => {
    const service = makeService()
    const methods = createProjectMethods(service, stubGetUserRole)
    const handler = findHandler(methods, 'project.getMembers')
    const result = await handler({ projectId: 'proj-1' }, makeCtx('u-1'))
    expect(service.getMembers).toHaveBeenCalledWith('proj-1')
    expect(result).toEqual([OWNER_MEMBER])
  })

  // ── 8. project.agentSpawn: throws AGENT_SPAWNER_NOT_AVAILABLE when no spawner ──

  it('project.agentSpawn: throws AGENT_SPAWNER_NOT_AVAILABLE when spawner not injected', async () => {
    const service = makeService()
    const methods = createProjectMethods(service, stubGetUserRole) // no spawner
    const handler = findHandler(methods, 'project.agentSpawn')
    await expect(
      handler({ projectId: 'proj-1', command: 'run' }, makeCtx('u-1'))
    ).rejects.toThrow('AGENT_SPAWNER_NOT_AVAILABLE')
  })

  // ── CR-TRACE-002/015: project.agentSpawn traceId propagation (TASK-BE-002.4/015.4) ──

  it('project.agentSpawn: forwards traceId from params into agentSpawner.spawn()', async () => {
    const service = makeService()
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, stubGetUserRole, spawner)
    const handler = findHandler(methods, 'project.agentSpawn')
    await handler({ projectId: 'proj-1', command: 'echo hi', traceId: 'resume-route-1' }, makeCtx('u-1'))
    expect(spawner.spawn).toHaveBeenCalledWith(
      expect.objectContaining({ traceId: 'resume-route-1' })
    )
  })

  // CR-TRACE-015 (TASK-BE-015.4): profile:agentSpawnRoute now wraps assertAccess and
  // ALWAYS forwards its own (fresh or resumed) span id as traceId into spawn() — so
  // agentOrch:spawn resumes the routing span rather than starting an unrelated one.
  // Absent client-provided traceId no longer means an undefined field downstream;
  // it means routeSpan generated a fresh id that spawn() then resumes into.
  it('project.agentSpawn: forwards a freshly-generated routeSpan traceId into agentSpawner.spawn() when the client did not provide one', async () => {
    const service = makeService()
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, stubGetUserRole, spawner)
    const handler = findHandler(methods, 'project.agentSpawn')
    await handler({ projectId: 'proj-1', command: 'echo hi' }, makeCtx('u-1'))
    expect(spawner.spawn).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: 'proj-1', userId: 'u-1' })
    )
    expect(vi.mocked(spawner.spawn).mock.calls[0]?.[0].traceId).toEqual(expect.any(String))
  })

  // ── CR-TRACE-015: profile:agentSpawnRoute tracing (TASK-BE-015.5) ──────────

  function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    return { events, stop: unregister }
  }

  it('project.agentSpawn: profile:agentSpawnRoute span.fail() on assertAccess rejection, spawn() never called', async () => {
    const service = makeService({
      assertAccess: vi.fn().mockRejectedValue(new Error('PROJECT_ACCESS_DENIED')),
    })
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, stubGetUserRole, spawner)
    const handler = findHandler(methods, 'project.agentSpawn')
    const { events, stop } = captureTraceEvents()

    await expect(
      handler({ projectId: 'proj-1', command: 'echo hi' }, makeCtx('u-1'))
    ).rejects.toThrow('PROJECT_ACCESS_DENIED')
    stop()

    const failEvent = events.find((e) => e.flow === 'profile:agentSpawnRoute' && e.level === 'fail')
    expect(failEvent?.fields.err).toContain('PROJECT_ACCESS_DENIED')
    expect(spawner.spawn).not.toHaveBeenCalled()
  })

  it('project.agentSpawn: profile:agentSpawnRoute resumes span id from params.traceId', async () => {
    const service = makeService()
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, stubGetUserRole, spawner)
    const handler = findHandler(methods, 'project.agentSpawn')
    const { events, stop } = captureTraceEvents()

    await handler({ projectId: 'proj-1', command: 'echo hi', traceId: 'resume-route-2' }, makeCtx('u-1'))
    stop()

    const routeEvents = events.filter((e) => e.flow === 'profile:agentSpawnRoute')
    expect(routeEvents.every((e) => e.id === 'resume-route-2')).toBe(true)
  })
})
