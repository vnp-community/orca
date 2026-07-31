/**
 * Tests for WorkspaceService (TDD-19) — T11
 *
 * Uses mocked relay bridge + mocked task/workflow/router.
 * Tests parallel init, offline tolerance, and git output parsers.
 *
 * Actual constructor:
 *   WorkspaceService(router, profileResolver, taskService, workflowOrchestrator, relayPool)
 *
 * teardownWorkspace: calls router.getProject(projectId) (no userId arg),
 *   then relayPool.release(project.devServerId)
 */

import { describe, it, expect, vi } from 'vitest'
import { WorkspaceService } from '../WorkspaceService'

// ── Mock helpers ──────────────────────────────────────────────────────────────

function makeMockRelay(overrides: Record<string, unknown> = {}) {
  return {
    call: vi.fn().mockImplementation(async (method: string) => {
      return overrides[method] ?? null
    }),
  }
}

function makeMockRouter(relay: ReturnType<typeof makeMockRelay> | null = null, projectDevServerId = 'srv-001') {
  return {
    getRelayForProject: vi.fn().mockResolvedValue(relay),
    getProject: vi.fn().mockResolvedValue(
      relay ? { id: 'proj-001', devServerId: projectDevServerId } : { id: 'proj-001', devServerId: projectDevServerId }
    ),
  }
}

function makeMockTaskService(tasks: object[] = []) {
  return {
    list: vi.fn().mockResolvedValue(tasks),
  }
}

function makeMockOrchestrator() {
  return {}
}

function makeMockRelayPool() {
  return {
    release: vi.fn(),
    getStatus: vi.fn().mockReturnValue({}),
  }
}

function makeMockProfileResolver() {
  return {
    resolve: vi.fn().mockResolvedValue({ _sources: {}, _resolvedAt: Date.now() }),
  }
}

// ── Simple git status output ───────────────────────────────────────────────────

const GIT_STATUS_OUTPUT = [
  '# branch.oid abc123',
  '# branch.head main',
  '# branch.upstream origin/main',
  '# branch.ab +2 -1',
  '1 M. 100644 100644 100644 a b src/file.ts',
  '? untracked.txt',
].join('\n')

const GIT_WORKTREE_OUTPUT = [
  'worktree /repo',
  'HEAD abc123',
  'branch refs/heads/main',
  '',
  'worktree /repo/.worktrees/feat',
  'HEAD def456',
  'branch refs/heads/feat/login',
  '',
].join('\n')

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('WorkspaceService', () => {
  // ── initWorkspace ──────────────────────────────────────────────────────────
  describe('initWorkspace', () => {
    it('fetches git status, worktrees, file tree, tasks in parallel', async () => {
      const relay = {
        call: vi.fn()
          .mockResolvedValueOnce({ stdout: GIT_STATUS_OUTPUT })  // git.exec status
          .mockResolvedValueOnce({ stdout: GIT_WORKTREE_OUTPUT }) // git.exec worktree
          .mockResolvedValueOnce([{ name: 'src', path: 'src', isDir: true }]), // fs.readDir
      }
      const pendingTask = { id: 't1', status: 'todo', title: 'A task' }
      const service = new WorkspaceService(
        makeMockRouter(relay as any) as any,
        makeMockProfileResolver() as any,
        makeMockTaskService([pendingTask]) as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      const result = await service.initWorkspace('proj-001', 'user-001')

      expect(result.gitStatus).not.toBeNull()
      expect(result.worktrees.length).toBeGreaterThan(0)
      expect(result.fileTree.length).toBeGreaterThan(0)
      expect(result.pendingTasks).toContainEqual(pendingTask)
    })

    it('returns null gitStatus when relay unavailable (offline tolerant)', async () => {
      const service = new WorkspaceService(
        makeMockRouter(null) as any,
        makeMockProfileResolver() as any,
        makeMockTaskService() as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      const result = await service.initWorkspace('proj-001', 'user-001')
      expect(result.gitStatus).toBeNull()
    })

    it('returns empty worktrees when relay git.exec fails (offline tolerant)', async () => {
      const relay = { call: vi.fn().mockRejectedValue(new Error('Connection refused')) }

      const service = new WorkspaceService(
        makeMockRouter(relay as any) as any,
        makeMockProfileResolver() as any,
        makeMockTaskService() as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      const result = await service.initWorkspace('proj-001', 'user-001')
      expect(result.worktrees).toEqual([])
      expect(result.fileTree).toEqual([])
    })

    it('returns empty pendingTasks when task list fails', async () => {
      const failTaskService = { list: vi.fn().mockRejectedValue(new Error('DB error')) }
      const service = new WorkspaceService(
        makeMockRouter(null) as any,
        makeMockProfileResolver() as any,
        failTaskService as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      const result = await service.initWorkspace('proj-001', 'user-001')
      expect(result.pendingTasks).toEqual([])
    })

    it('filters tasks to only todo/in_progress/blocked', async () => {
      const tasks = [
        { id: '1', status: 'todo' },
        { id: '2', status: 'done' },
        { id: '3', status: 'in_progress' },
        { id: '4', status: 'blocked' },
        { id: '5', status: 'backlog' },
      ]
      const service = new WorkspaceService(
        makeMockRouter(null) as any,
        makeMockProfileResolver() as any,
        makeMockTaskService(tasks) as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      const result = await service.initWorkspace('proj-001', 'user-001')
      expect(result.pendingTasks.map((t: any) => t.status).sort()).toEqual(['blocked', 'in_progress', 'todo'].sort())
    })
  })

  // ── teardownWorkspace ──────────────────────────────────────────────────────
  describe('teardownWorkspace', () => {
    it('calls relayPool.release with project devServerId', async () => {
      const relayPool = makeMockRelayPool()
      const service = new WorkspaceService(
        makeMockRouter(null, 'srv-XYZ') as any,
        makeMockProfileResolver() as any,
        makeMockTaskService() as any,
        makeMockOrchestrator() as any,
        relayPool as any
      )
      await service.teardownWorkspace('proj-001')
      expect(relayPool.release).toHaveBeenCalledWith('srv-XYZ')
    })

    it('is non-fatal when getProject throws', async () => {
      const router = {
        getProject: vi.fn().mockRejectedValue(new Error('Not found')),
        getRelayForProject: vi.fn()
      }
      const service = new WorkspaceService(
        router as any,
        makeMockProfileResolver() as any,
        makeMockTaskService() as any,
        makeMockOrchestrator() as any,
        makeMockRelayPool() as any
      )
      await expect(service.teardownWorkspace('proj-001')).resolves.toBeUndefined()
    })
  })

  // ── parseGitStatus ─────────────────────────────────────────────────────────
  describe('parseGitStatus', () => {
    it('parses branch.head from porcelain v2', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const status = service.parseGitStatus(GIT_STATUS_OUTPUT)
      expect(status.branch).toBe('main')
    })

    it('parses ahead/behind counts', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const status = service.parseGitStatus(GIT_STATUS_OUTPUT)
      expect(status.ahead).toBe(2)
      expect(status.behind).toBe(1)
    })

    it('parses upstream reference', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const status = service.parseGitStatus(GIT_STATUS_OUTPUT)
      expect(status.upstream).toBe('origin/main')
    })

    it('counts staged files correctly', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const status = service.parseGitStatus(GIT_STATUS_OUTPUT)
      expect(status.staged).toBe(1)
    })

    it('counts untracked files (? prefix)', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const status = service.parseGitStatus(GIT_STATUS_OUTPUT)
      expect(status.untracked).toBe(1)
    })
  })

  // ── parseWorktreeList ──────────────────────────────────────────────────────
  describe('parseWorktreeList', () => {
    it('parses worktree path and branch', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const worktrees = service.parseWorktreeList(GIT_WORKTREE_OUTPUT)
      expect(worktrees[0].path).toBe('/repo')
      expect(worktrees[0].branch).toBe('main')
    })

    it('marks first worktree as isMain=true', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const worktrees = service.parseWorktreeList(GIT_WORKTREE_OUTPUT)
      expect(worktrees[0].isMain).toBe(true)
      expect(worktrees[1].isMain).toBe(false)
    })

    it('parses branch name without refs/heads/ prefix', () => {
      const service = new WorkspaceService({} as any, {} as any, {} as any, {} as any, {} as any)
      const worktrees = service.parseWorktreeList(GIT_WORKTREE_OUTPUT)
      expect(worktrees[1].branch).toBe('feat/login')
    })
  })
})
