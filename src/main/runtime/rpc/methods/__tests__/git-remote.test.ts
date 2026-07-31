/**
 * Tests for git-remote.ts server-side RPC methods (TASK-044) — ≥ 17 tests
 *
 * Uses mocks for relay, projectRouter, taskService, taskGrantService.
 *
 * @module main/runtime/rpc/methods/__tests__/git-remote.test
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { registerRemoteGitRpcMethods } from '../git-remote'
import type { RpcMethod, RpcContext } from '../../core'
import type { ProjectServerRouter } from '../../../../project/ProjectServerRouter'
import type { AIProviderService } from '../../../../ai-providers/AIProviderService'
import type { TaskService } from '../../../../task/TaskService'
import type { TaskGrantService } from '../../../../task/TaskGrantService'

// ── Mock factories ─────────────────────────────────────────────────────────────

function makeRelay(callResults: Record<string, unknown> = {}) {
  return {
    call: vi.fn().mockImplementation((method: string) => {
      return Promise.resolve(callResults[method] ?? { stdout: '', stderr: '', exitCode: 0 })
    }),
    callStream: vi.fn().mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 }),
  }
}

function makeRouter(relay = makeRelay()) {
  return {
    getRelayForProject: vi.fn().mockResolvedValue(relay),
  } as unknown as ProjectServerRouter
}

function makeTaskService() {
  return {
    get: vi.fn().mockResolvedValue(null),
    update: vi.fn().mockResolvedValue(undefined),
    addComment: vi.fn().mockResolvedValue(undefined),
    list: vi.fn().mockResolvedValue([]),
  } as unknown as TaskService
}

function makeTaskGrantService() {
  return {
    resolvePermission: vi.fn().mockResolvedValue('edit'),
  } as unknown as TaskGrantService
}

function makeAIService() {
  return {} as unknown as AIProviderService
}

function makeCtx(userId = 'user-1'): RpcContext {
  return { userId } as RpcContext
}

function findMethod(methods: RpcMethod[], name: string) {
  const m = methods.find(m => m.name === name)
  if (!m) throw new Error(`Method ${name} not found`)
  return m
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('git-remote RPC methods', () => {
  let relay: ReturnType<typeof makeRelay>
  let router: ProjectServerRouter
  let taskService: TaskService
  let taskGrantService: TaskGrantService
  let methods: RpcMethod[]

  beforeEach(() => {
    relay = makeRelay({
      'git.exec': { stdout: 'mock output', stderr: '', exitCode: 0 },
      'git.execStream': { stdout: 'stream output', stderr: '', exitCode: 0 },
    })
    router = makeRouter(relay)
    taskService = makeTaskService()
    taskGrantService = makeTaskGrantService()
    methods = registerRemoteGitRpcMethods(router, makeAIService(), taskService, taskGrantService)
  })

  // 1. git.status → relay.call with correct args ───────────────────────────────

  it('git.status → calls relay git.exec with status --porcelain=v2 --branch', async () => {
    const method = findMethod(methods, 'git.status')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['status', '--porcelain=v2', '--branch']),
    }))
  })

  // 2. git.diff staged=true adds --staged ─────────────────────────────────────

  it('git.diff staged=true → adds --staged to args', async () => {
    const method = findMethod(methods, 'git.diff')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', staged: true }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['diff', '--staged']),
    }))
  })

  it('git.diff staged=false → no --staged in args', async () => {
    const method = findMethod(methods, 'git.diff')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', staged: false }, makeCtx())
    const callArgs = vi.mocked(relay.call).mock.calls[0]
    const args = (callArgs?.[1] as { args?: string[] })?.args ?? []
    expect(args).not.toContain('--staged')
  })

  // 3. git.add → files list passed ────────────────────────────────────────────

  it('git.add → files list passed with -- separator', async () => {
    const method = findMethod(methods, 'git.add')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', files: ['src/a.ts', 'src/b.ts'] }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['add', '--', 'src/a.ts', 'src/b.ts']),
    }))
  })

  // 4. git.commit → message passed correctly ──────────────────────────────────

  it('git.commit → message passed correctly', async () => {
    const method = findMethod(methods, 'git.commit')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', message: 'fix: test' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['commit', '-m', 'fix: test']),
    }))
  })

  // 5. git.commit with #TG-xxx → task auto-advance triggered ──────────────────

  it('git.commit with #TG-xxx → autoAdvanceTasks tries to get task', async () => {
    const method = findMethod(methods, 'git.commit')
    vi.mocked(taskService.get).mockResolvedValue({
      id: 'abc123', status: 'in_progress', title: 'Test task',
    } as any)
    vi.mocked(taskGrantService.resolvePermission).mockResolvedValue('edit')

    await method.handler({
      projectId: 'proj-1', worktreePath: '/repo',
      message: 'fix: done #TG-abc123',
    }, makeCtx())

    // Wait a tick for the non-awaited auto-advance
    await new Promise(r => setTimeout(r, 10))
    expect(taskService.get).toHaveBeenCalledWith('abc123')
  })

  // 6. git.commit without #TG-xxx → no task update ────────────────────────────

  it('git.commit without task ref → taskService.get not called', async () => {
    const method = findMethod(methods, 'git.commit')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', message: 'chore: cleanup' }, makeCtx())
    await new Promise(r => setTimeout(r, 10))
    expect(taskService.get).not.toHaveBeenCalled()
  })

  // 7. git.push → relay.callStream used ───────────────────────────────────────

  it('git.push → calls git.execStream on relay', async () => {
    const method = findMethod(methods, 'git.push')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.execStream', expect.objectContaining({
      args: expect.arrayContaining(['push']),
    }))
  })

  // 8. git.pull → relay.callStream used ───────────────────────────────────────

  it('git.pull → calls git.execStream on relay', async () => {
    const method = findMethod(methods, 'git.pull')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.execStream', expect.objectContaining({
      args: expect.arrayContaining(['pull']),
    }))
  })

  // 9. git.generateCommitMessage → empty staged diff → GIT_NO_STAGED_CHANGES ──

  it('git.generateCommitMessage → empty staged diff → throws GIT_NO_STAGED_CHANGES', async () => {
    relay = makeRelay({ 'git.exec': { stdout: '', stderr: '', exitCode: 0 } })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), taskService, taskGrantService)

    const method = findMethod(methods, 'git.generateCommitMessage')
    await expect(
      method.handler({ projectId: 'proj-1', worktreePath: '/repo', devServerId: 'ds-1' }, makeCtx())
    ).rejects.toThrow('GIT_NO_STAGED_CHANGES')
  })

  // 10. git.generateCommitMessage → diff → AI called → returns message ─────────

  it('git.generateCommitMessage → has diff → calls ai.complete and returns message', async () => {
    relay = makeRelay({
      'git.exec': { stdout: '+changed line\n-removed line\n', stderr: '', exitCode: 0 },
      'ai.complete': { content: 'feat: add workspace service' },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), taskService, taskGrantService)

    const method = findMethod(methods, 'git.generateCommitMessage')
    const result = await method.handler(
      { projectId: 'proj-1', worktreePath: '/repo', devServerId: 'ds-1' }, makeCtx()
    ) as { message: string }

    expect(result.message).toBe('feat: add workspace service')
    expect(relay.call).toHaveBeenCalledWith('ai.complete', expect.objectContaining({ format: 'text' }))
  })

  // 11. git.branch.list → relay called with branch --format ─────────────────

  it('git.branch.list → calls git.exec with branch and format args', async () => {
    const method = findMethod(methods, 'git.branch.list')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['branch']),
    }))
  })

  // 12. git.branch.create → name passed ───────────────────────────────────────

  it('git.branch.create → creates branch with correct name', async () => {
    const method = findMethod(methods, 'git.branch.create')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', name: 'feature/new' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['branch', 'feature/new']),
    }))
  })

  // 13. git.branch.delete force=false → -d flag ───────────────────────────────

  it('git.branch.delete force=false → uses -d flag', async () => {
    const method = findMethod(methods, 'git.branch.delete')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', name: 'old-branch', force: false }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['-d', 'old-branch']),
    }))
  })

  // 14. git.worktree.list → correct args ───────────────────────────────────────

  it('git.worktree.list → calls git.exec with worktree list --porcelain', async () => {
    const method = findMethod(methods, 'git.worktree.list')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['worktree', 'list', '--porcelain']),
    }))
  })

  // 15. git.log → --oneline and -N limit ─────────────────────────────────────

  it('git.log → calls git.exec with --oneline and limit', async () => {
    const method = findMethod(methods, 'git.log')
    await method.handler({ projectId: 'proj-1', worktreePath: '/repo', limit: 10 }, makeCtx())
    expect(relay.call).toHaveBeenCalledWith('git.exec', expect.objectContaining({
      args: expect.arrayContaining(['log', '--oneline', '-10']),
    }))
  })

  // 16. Task auto-advance: has edit perm → status='review' ────────────────────

  it('task auto-advance: has edit perm and task in_progress → update to review', async () => {
    // Re-create methods with fresh mocks that have the right permission
    const freshGrantService = makeTaskGrantService()
    const freshTaskService = makeTaskService()

    vi.mocked(freshGrantService.resolvePermission).mockResolvedValue('edit')
    vi.mocked(freshTaskService.get).mockResolvedValue({
      id: 'task-abc', status: 'in_progress', title: 'Fix it',
    } as any)

    const freshRelay = makeRelay({
      'git.exec': { stdout: '', stderr: '', exitCode: 0 },
    })
    const freshRouter = makeRouter(freshRelay)
    const freshMethods = registerRemoteGitRpcMethods(freshRouter, makeAIService(), freshTaskService, freshGrantService)

    const method = findMethod(freshMethods, 'git.commit')
    await method.handler({
      projectId: 'proj-1', worktreePath: '/repo',
      message: 'fix: done #TG-task-abc',
    }, makeCtx())
    // Wait for non-awaited auto-advance Promise.allSettled
    await new Promise(r => setTimeout(r, 50))

    expect(freshTaskService.update).toHaveBeenCalledWith('task-abc', { status: 'review' })
  })

  // 17. Task auto-advance: no perm → no status change ─────────────────────────

  it('task auto-advance: no permission → update not called', async () => {
    vi.mocked(taskGrantService.resolvePermission).mockResolvedValue('view') // below edit
    vi.mocked(taskService.get).mockResolvedValue({ id: 'task-xyz', status: 'todo' } as any)

    const method = findMethod(methods, 'git.commit')
    await method.handler({
      projectId: 'proj-1', worktreePath: '/repo',
      message: 'fix: close #TG-task-xyz',
    }, makeCtx())
    await new Promise(r => setTimeout(r, 20))

    expect(taskService.update).not.toHaveBeenCalled()
  })
})
