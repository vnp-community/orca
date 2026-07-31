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

/** Find a method handler by name from the factory output */
function findHandler(methods: ReturnType<typeof createProjectMethods>, name: string) {
  const method = methods.find(m => m.name === name)
  if (!method) throw new Error(`Method not found: ${name}`)
  return method.handler
}

describe('project RPC methods', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── 1. project.list → returns user projects ───────────────────────────────

  it('project.list: returns user projects via service.list', async () => {
    const service = makeService()
    const methods = createProjectMethods(service)
    const handler = findHandler(methods, 'project.list')
    const result = await handler(undefined, makeCtx('u-1'))
    expect(service.list).toHaveBeenCalledWith('u-1')
    expect(result).toEqual([FAKE_PROJECT])
  })

  // ── 2. project.create → delegates to service, returns new project ─────────

  it('project.create: delegates to service with createdBy from userId', async () => {
    const service = makeService()
    const methods = createProjectMethods(service)
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
    const methods = createProjectMethods(service)
    const handler = findHandler(methods, 'project.addMember')
    await expect(
      handler({ projectId: 'proj-1', userId: 'u-2', role: 'member' }, makeCtx('u-viewer'))
    ).rejects.toThrow('FORBIDDEN')
  })

  // ── 4. project.agentSpawn → calls spawner.spawn ───────────────────────────

  it('project.agentSpawn: calls agentSpawner.spawn with userId and projectId', async () => {
    const service = makeService()
    const spawner = makeSpawner()
    const methods = createProjectMethods(service, spawner)
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
    const methods = createProjectMethods(service)
    const handler = findHandler(methods, 'project.delete')
    await expect(
      handler({ projectId: 'proj-1' }, makeCtx('u-viewer'))
    ).rejects.toThrow('FORBIDDEN')
  })

  // ── 6. project.list: throws UNAUTHENTICATED when no userId ───────────────

  it('project.list: throws UNAUTHENTICATED when ctx has no userId', async () => {
    const service = makeService()
    const methods = createProjectMethods(service)
    const handler = findHandler(methods, 'project.list')
    await expect(handler(undefined, {} as RpcContext)).rejects.toThrow('UNAUTHENTICATED')
  })

  // ── 7. project.getMembers: delegates to service.getMembers ───────────────

  it('project.getMembers: returns members for valid member', async () => {
    const service = makeService()
    const methods = createProjectMethods(service)
    const handler = findHandler(methods, 'project.getMembers')
    const result = await handler({ projectId: 'proj-1' }, makeCtx('u-1'))
    expect(service.getMembers).toHaveBeenCalledWith('proj-1')
    expect(result).toEqual([OWNER_MEMBER])
  })

  // ── 8. project.agentSpawn: throws AGENT_SPAWNER_NOT_AVAILABLE when no spawner ──

  it('project.agentSpawn: throws AGENT_SPAWNER_NOT_AVAILABLE when spawner not injected', async () => {
    const service = makeService()
    const methods = createProjectMethods(service) // no spawner
    const handler = findHandler(methods, 'project.agentSpawn')
    await expect(
      handler({ projectId: 'proj-1', command: 'run' }, makeCtx('u-1'))
    ).rejects.toThrow('AGENT_SPAWNER_NOT_AVAILABLE')
  })
})
