/**
 * git-remote-rpc.test.ts — Tests cho createGitRemoteV6Methods (TDD-20)
 *
 * Test strategy:
 * - Mock ProjectServerRouter.getRelayForProject()
 * - Mock relay.call() để verify routing đúng method + params
 * - Test param forwarding: mỗi RPC method forward đúng params
 *
 * Note: getRelayForProject(projectId, userId) — 2 args
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createGitRemoteV6Methods } from '../git-remote-rpc'
import type { RpcContext } from '../../core'

// ── Helpers ───────────────────────────────────────────────────────────────────

const mockRelayCall = vi.fn().mockResolvedValue({ ok: true })
const mockGetRelayForProject = vi.fn().mockResolvedValue({
  call: mockRelayCall,
})
const mockProjectRouter = {
  getRelayForProject: mockGetRelayForProject,
}

function makeCtx(userId = 'user-001'): RpcContext {
  return { userId, user: { id: userId, role: 'developer' } } as unknown as RpcContext
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('createGitRemoteV6Methods', () => {
  let methods: ReturnType<typeof createGitRemoteV6Methods>

  beforeEach(() => {
    vi.clearAllMocks()
    mockRelayCall.mockResolvedValue({ ok: true })
    methods = createGitRemoteV6Methods(mockProjectRouter as any)
  })

  // ── Structure ─────────────────────────────────────────────────────────────────
  it('returns array của RPC methods', () => {
    expect(Array.isArray(methods)).toBe(true)
    expect(methods.length).toBeGreaterThan(0)
  })

  it('method names đúng format (git.*)', () => {
    const names = methods.map(m => m.name)
    expect(names).toContain('git.status')
    expect(names).toContain('git.diff')
    expect(names).toContain('git.add')
    expect(names).toContain('git.restore')
    expect(names).toContain('git.commit')
    expect(names).toContain('git.push')
    expect(names).toContain('git.pull')
    expect(names).toContain('git.branch.list')
    expect(names).toContain('git.checkout')
  })

  // ── git.status ────────────────────────────────────────────────────────────────
  describe('git.status method', () => {
    it('gọi getRelayForProject với đúng projectId + userId', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1' }, makeCtx('user-001'))
      expect(mockGetRelayForProject).toHaveBeenCalledWith('proj-1', 'user-001')
    })

    it('route tới relay.call git.status', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1' }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.status', expect.any(Object))
    })

    it('forward worktreePath nếu có', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1', worktreePath: '/wt' }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.status', expect.objectContaining({ cwd: '/wt' }))
    })

    it('throw nếu projectId không tìm được relay', async () => {
      mockGetRelayForProject.mockRejectedValueOnce(new Error('Project not found'))
      const method = methods.find(m => m.name === 'git.status')!
      await expect(method.handler({ projectId: 'bad' }, makeCtx())).rejects.toThrow('Project not found')
    })
  })

  // ── git.add ───────────────────────────────────────────────────────────────────
  describe('git.add method', () => {
    it('forward files array tới relay', async () => {
      const method = methods.find(m => m.name === 'git.add')!
      await method.handler({ projectId: 'proj-1', files: ['a.ts', 'b.ts'] }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.add', expect.objectContaining({ files: ['a.ts', 'b.ts'] }))
    })

    it('throw nếu projectId không tìm được relay', async () => {
      mockGetRelayForProject.mockRejectedValueOnce(new Error('Project not found'))
      const method = methods.find(m => m.name === 'git.add')!
      await expect(method.handler({ projectId: 'bad', files: [] }, makeCtx())).rejects.toThrow('Project not found')
    })
  })

  // ── git.diff ───────────────────────────────────────────────────────────────────
  describe('git.diff method', () => {
    it('forward staged=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.diff')!
      await method.handler({ projectId: 'proj-1', staged: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.diff', expect.objectContaining({ staged: true }))
    })

    it('forward file path tới relay', async () => {
      const method = methods.find(m => m.name === 'git.diff')!
      await method.handler({ projectId: 'proj-1', file: 'src/main.ts' }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.diff', expect.objectContaining({ file: 'src/main.ts' }))
    })
  })

  // ── git.commit ─────────────────────────────────────────────────────────────────
  describe('git.commit method', () => {
    it('forward message tới relay', async () => {
      const method = methods.find(m => m.name === 'git.commit')!
      await method.handler({ projectId: 'proj-1', message: 'feat: update' }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.commit', expect.objectContaining({ message: 'feat: update' }))
    })
  })

  // ── git.push ──────────────────────────────────────────────────────────────────
  describe('git.push method', () => {
    it('forward force=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.push')!
      await method.handler({ projectId: 'proj-1', force: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.push', expect.objectContaining({ force: true }))
    })

    it('forward remote + branch tới relay', async () => {
      const method = methods.find(m => m.name === 'git.push')!
      await method.handler({ projectId: 'proj-1', remote: 'upstream', branch: 'main' }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.push', expect.objectContaining({ remote: 'upstream', branch: 'main' }))
    })
  })

  // ── git.pull ──────────────────────────────────────────────────────────────────
  describe('git.pull method', () => {
    it('forward rebase=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.pull')!
      await method.handler({ projectId: 'proj-1', rebase: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.pull', expect.objectContaining({ rebase: true }))
    })
  })

  // ── git.branch.list ────────────────────────────────────────────────────────────
  describe('git.branch.list method', () => {
    it('forward remote=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.branch.list')!
      await method.handler({ projectId: 'proj-1', remote: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.branch.list', expect.objectContaining({ remote: true }))
    })
  })

  // ── git.checkout ──────────────────────────────────────────────────────────────
  describe('git.checkout method', () => {
    it('forward branch + create tới relay', async () => {
      const method = methods.find(m => m.name === 'git.checkout')!
      await method.handler({ projectId: 'proj-1', branch: 'feat/new', create: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.checkout', expect.objectContaining({ branch: 'feat/new', create: true }))
    })
  })

  // ── git.restore ────────────────────────────────────────────────────────────────
  describe('git.restore method', () => {
    it('forward staged=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.restore')!
      await method.handler({ projectId: 'proj-1', files: ['x.ts'], staged: true }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.restore', expect.objectContaining({ staged: true }))
    })

    it('forward files array tới relay', async () => {
      const method = methods.find(m => m.name === 'git.restore')!
      await method.handler({ projectId: 'proj-1', files: ['src/a.ts', 'src/b.ts'] }, makeCtx())
      expect(mockRelayCall).toHaveBeenCalledWith('git.restore', expect.objectContaining({ files: ['src/a.ts', 'src/b.ts'] }))
    })
  })
})
