/**
 * Tests for ProjectServerRouter (TDD-15) — TASK-018
 *
 * Uses mocks for ProjectService, DevServerManager, RelayConnectionPool.
 * ≥ 10 tests.
 *
 * @module main/project/__tests__/ProjectServerRouter.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { ProjectServerRouter } from '../ProjectServerRouter'
import type { ProjectService } from '../ProjectService'
import type { DevServerManager } from '../../dev-server/dev-server-manager'
import type { RelayConnectionPool } from '../../dev-server/relay-connection-pool'
import type { ProfileResolver } from '../../profile/ProfileResolver'
import type { ProjectMember, OrcaProject } from '../../../shared/project-types'

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

const FAKE_MEMBER: ProjectMember = {
  projectId: 'proj-1',
  userId: 'u-1',
  role: 'owner',
  addedAt: new Date(),
}

const FAKE_SERVER = { id: 'srv-1', name: 'Dev Server', connectionType: 'direct-websocket' as const }

const FAKE_RELAY = { call: vi.fn().mockResolvedValue({ sessionId: 'agent-1' }) }

function makeRouter(overrides: {
  projectService?: Partial<ProjectService>
  devServerManager?: Partial<DevServerManager>
  relayPool?: Partial<RelayConnectionPool>
} = {}): ProjectServerRouter {
  const projectService = {
    assertAccess: vi.fn().mockResolvedValue(FAKE_MEMBER),
    get: vi.fn().mockResolvedValue(FAKE_PROJECT),
    ...overrides.projectService,
  } as unknown as ProjectService

  const devServerManager = {
    get: vi.fn().mockReturnValue(FAKE_SERVER),
    ...overrides.devServerManager,
  } as unknown as DevServerManager

  const relayPool = {
    getOrConnect: vi.fn().mockResolvedValue(FAKE_RELAY),
    ...overrides.relayPool,
  } as unknown as RelayConnectionPool

  return new ProjectServerRouter(projectService, devServerManager, relayPool)
}

function makeProfileResolver(profile = {}): ProfileResolver {
  return {
    resolve: vi.fn().mockResolvedValue({
      _sources: {},
      _resolvedAt: Date.now(),
      ...profile,
    }),
    invalidate: vi.fn(),
  } as unknown as ProfileResolver
}

describe('ProjectServerRouter', () => {
  afterEach(() => { vi.restoreAllMocks() })

  // ── 1. getRelayForProject: valid member → relay returned ──────────────────

  it('getRelayForProject: returns relay from pool for valid member', async () => {
    const router = makeRouter()
    const relay = await router.getRelayForProject('proj-1', 'u-1')
    expect(relay).toBe(FAKE_RELAY)
  })

  // ── 2. getRelayForProject: non-member → PROJECT_ACCESS_DENIED ────────────

  it('getRelayForProject: propagates PROJECT_ACCESS_DENIED for non-member', async () => {
    const router = makeRouter({
      projectService: {
        assertAccess: vi.fn().mockRejectedValue(new Error('PROJECT_ACCESS_DENIED')),
        get: vi.fn().mockResolvedValue(FAKE_PROJECT),
      }
    })
    await expect(router.getRelayForProject('proj-1', 'stranger')).rejects.toThrow('PROJECT_ACCESS_DENIED')
  })

  // ── 3. getRelayForProject: project not found → PROJECT_NOT_FOUND ──────────

  it('getRelayForProject: throws PROJECT_NOT_FOUND when project missing', async () => {
    const router = makeRouter({
      projectService: {
        assertAccess: vi.fn().mockResolvedValue(FAKE_MEMBER),
        get: vi.fn().mockResolvedValue(null),
      }
    })
    await expect(router.getRelayForProject('no-proj', 'u-1')).rejects.toThrow('PROJECT_NOT_FOUND')
  })

  // ── 4. getRelayForProject: dev server not found → DEV_SERVER_NOT_FOUND ────

  it('getRelayForProject: throws DEV_SERVER_NOT_FOUND when server missing', async () => {
    const router = makeRouter({
      devServerManager: { get: vi.fn().mockReturnValue(null) }
    })
    await expect(router.getRelayForProject('proj-1', 'u-1')).rejects.toThrow('DEV_SERVER_NOT_FOUND')
  })

  // ── 5. getProjectContext: all fields populated ────────────────────────────

  it('getProjectContext: returns all context fields', async () => {
    const router = makeRouter()
    const profileResolver = makeProfileResolver()
    const ctx = await router.getProjectContext('proj-1', 'u-1', profileResolver)
    expect(ctx.project).toBe(FAKE_PROJECT)
    expect(ctx.member).toBe(FAKE_MEMBER)
    expect(ctx.devServer).toBe(FAKE_SERVER)
    expect(ctx.resolvedProfile).toBeDefined()
    expect(ctx.resolvedProfile._resolvedAt).toBeTypeOf('number')
  })

  // ── 6. getProjectContext: profileResolver.resolve called with userId ───────

  it('getProjectContext: calls profileResolver.resolve with userId', async () => {
    const router = makeRouter()
    const profileResolver = makeProfileResolver()
    await router.getProjectContext('proj-1', 'u-1', profileResolver)
    expect(profileResolver.resolve).toHaveBeenCalledWith('u-1')
  })

  // ── 7. getProject: delegates to projectService.get ────────────────────────

  it('getProject: delegates to projectService.get', async () => {
    const mockGet = vi.fn().mockResolvedValue(FAKE_PROJECT)
    const router = makeRouter({
      projectService: {
        assertAccess: vi.fn().mockResolvedValue(FAKE_MEMBER),
        get: mockGet,
      }
    })
    const result = await router.getProject('proj-1')
    expect(mockGet).toHaveBeenCalledWith('proj-1')
    expect(result).toBe(FAKE_PROJECT)
  })

  // ── 8. getRelayForProject: calls relayPool.getOrConnect with correct args ──

  it('getRelayForProject: calls relayPool.getOrConnect with devServerId + server', async () => {
    const mockGetOrConnect = vi.fn().mockResolvedValue(FAKE_RELAY)
    const router = makeRouter({ relayPool: { getOrConnect: mockGetOrConnect } })
    await router.getRelayForProject('proj-1', 'u-1')
    expect(mockGetOrConnect).toHaveBeenCalledWith('srv-1', FAKE_SERVER)
  })

  // ── 9. getProjectContext: throws PROJECT_NOT_FOUND when project null ────────

  it('getProjectContext: throws PROJECT_NOT_FOUND when project missing', async () => {
    const router = makeRouter({
      projectService: {
        assertAccess: vi.fn().mockResolvedValue(FAKE_MEMBER),
        get: vi.fn().mockResolvedValue(null),
      }
    })
    const profileResolver = makeProfileResolver()
    await expect(router.getProjectContext('no-proj', 'u-1', profileResolver)).rejects.toThrow('PROJECT_NOT_FOUND')
  })

  // ── 10. getProjectContext: throws DEV_SERVER_NOT_FOUND ────────────────────

  it('getProjectContext: throws DEV_SERVER_NOT_FOUND when server missing', async () => {
    const router = makeRouter({
      devServerManager: { get: vi.fn().mockReturnValue(null) }
    })
    const profileResolver = makeProfileResolver()
    await expect(router.getProjectContext('proj-1', 'u-1', profileResolver)).rejects.toThrow('DEV_SERVER_NOT_FOUND')
  })
})
