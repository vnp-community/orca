/**
 * git-handler-v6.test.ts — Tests cho gitRemoteHandlersV6 (TDD-20)
 *
 * Test strategy:
 * - Mock git-remote-handler để tránh real git calls
 * - Test params mapping: mỗi method gọi đúng git args
 * - Test result parsing: branches, ok status
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the v5 base handlers that v6 depends on
vi.mock('../git-remote-handler', async () => {
  const mod = await vi.importActual('../git-remote-handler') as typeof import('../git-remote-handler')
  return {
    ...mod,
    gitRemoteHandlers: {
      'git.exec': vi.fn().mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 }),
      'git.execStream': vi.fn().mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 }),
    },
  }
})

// Import after mock is set up
import { gitRemoteHandlersV6 } from '../git-remote-handler-v6'

describe('gitRemoteHandlersV6', () => {
  beforeEach(() => {
    vi.mocked(gitRemoteHandlersV6['git.exec']).mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 })
  })

  // ── Kế thừa v5 ──────────────────────────────────────────────────────────────
  describe('kế thừa v5', () => {
    it('có git.exec từ v5', () => {
      expect(gitRemoteHandlersV6['git.exec']).toBeDefined()
    })

    it('có git.execStream từ v5', () => {
      expect(gitRemoteHandlersV6['git.execStream']).toBeDefined()
    })
  })

  // ── git.status ────────────────────────────────────────────────────────────────
  describe('git.status', () => {
    it('gọi với porcelain v2 format', async () => {
      await gitRemoteHandlersV6['git.status']({ cwd: '/repo' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['status', '--porcelain=v2', '--branch'] })
      )
    })

    it('dùng worktreePath nếu có', async () => {
      await gitRemoteHandlersV6['git.status']({ cwd: '/repo', worktreePath: '/worktree' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ cwd: '/worktree' })
      )
    })

    it('dùng cwd khi worktreePath không có', async () => {
      await gitRemoteHandlersV6['git.status']({ cwd: '/main-repo' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ cwd: '/main-repo' })
      )
    })

    it('trả về { raw: stdout }', async () => {
      vi.mocked(gitRemoteHandlersV6['git.exec']).mockResolvedValueOnce({ stdout: '# branch.head main\n', stderr: '', exitCode: 0 })
      const result = await gitRemoteHandlersV6['git.status']({ cwd: '/repo' })
      expect(result).toHaveProperty('raw')
    })
  })

  // ── git.diff ──────────────────────────────────────────────────────────────────
  describe('git.diff', () => {
    it('gọi git diff không staged', async () => {
      await gitRemoteHandlersV6['git.diff']({ cwd: '/repo' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['diff'] })
      )
    })

    it('gọi git diff --staged khi staged=true', async () => {
      await gitRemoteHandlersV6['git.diff']({ cwd: '/repo', staged: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['--staged']) })
      )
    })

    it('thêm -- file khi có file param', async () => {
      await gitRemoteHandlersV6['git.diff']({ cwd: '/repo', file: 'src/foo.ts' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['--', 'src/foo.ts']) })
      )
    })
  })

  // ── git.add ───────────────────────────────────────────────────────────────────
  describe('git.add', () => {
    it('gọi git add với files và trả về { ok: true }', async () => {
      const result = await gitRemoteHandlersV6['git.add']({ cwd: '/repo', files: ['a.ts', 'b.ts'] })
      expect(result).toEqual({ ok: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['add', '--', 'a.ts', 'b.ts'] })
      )
    })
  })

  // ── git.restore ────────────────────────────────────────────────────────────────
  describe('git.restore', () => {
    it('restore unstaged files — trả về { ok: true }', async () => {
      const result = await gitRemoteHandlersV6['git.restore']({ cwd: '/repo', files: ['foo.ts'] })
      expect(result).toEqual({ ok: true })
    })

    it('restore staged files khi staged=true', async () => {
      await gitRemoteHandlersV6['git.restore']({ cwd: '/repo', files: ['foo.ts'], staged: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['--staged']) })
      )
    })
  })

  // ── git.commit ─────────────────────────────────────────────────────────────────
  describe('git.commit', () => {
    it('gọi git commit -m message và trả về { ok, output }', async () => {
      vi.mocked(gitRemoteHandlersV6['git.exec']).mockResolvedValueOnce({ stdout: 'main 1 commit', stderr: '', exitCode: 0 })
      const result = await gitRemoteHandlersV6['git.commit']({ cwd: '/repo', message: 'feat: add feature' })
      expect(result).toHaveProperty('ok', true)
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['commit', '-m', 'feat: add feature'] })
      )
    })
  })

  // ── git.push ──────────────────────────────────────────────────────────────────
  describe('git.push', () => {
    it('gọi git push origin HEAD mặc định', async () => {
      await gitRemoteHandlersV6['git.push']({ cwd: '/repo' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['push', 'origin', 'HEAD'] })
      )
    })

    it('thêm --force-with-lease khi force=true', async () => {
      await gitRemoteHandlersV6['git.push']({ cwd: '/repo', force: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['--force-with-lease']) })
      )
    })

    it('dùng custom remote và branch', async () => {
      await gitRemoteHandlersV6['git.push']({ cwd: '/repo', remote: 'upstream', branch: 'feat/new' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['push', 'upstream', 'feat/new'] })
      )
    })
  })

  // ── git.pull ──────────────────────────────────────────────────────────────────
  describe('git.pull', () => {
    it('gọi git pull origin mặc định', async () => {
      await gitRemoteHandlersV6['git.pull']({ cwd: '/repo' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['pull', 'origin'] })
      )
    })

    it('thêm --rebase khi rebase=true', async () => {
      await gitRemoteHandlersV6['git.pull']({ cwd: '/repo', rebase: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['--rebase']) })
      )
    })
  })

  // ── git.branch.list ────────────────────────────────────────────────────────────
  describe('git.branch.list', () => {
    it('parse branches từ stdout', async () => {
      vi.mocked(gitRemoteHandlersV6['git.exec']).mockResolvedValueOnce({
        stdout: 'main\nfeature/foo\nfix/bar\n',
        stderr: '',
        exitCode: 0,
      })
      const result = await gitRemoteHandlersV6['git.branch.list']({ cwd: '/repo' })
      expect(result.branches).toEqual(['main', 'feature/foo', 'fix/bar'])
    })

    it('thêm -r cho remote branches', async () => {
      await gitRemoteHandlersV6['git.branch.list']({ cwd: '/repo', remote: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: expect.arrayContaining(['-r']) })
      )
    })

    it('lọc bỏ empty lines trong kết quả', async () => {
      vi.mocked(gitRemoteHandlersV6['git.exec']).mockResolvedValueOnce({
        stdout: 'main\n\nfeat\n',
        stderr: '',
        exitCode: 0,
      })
      const result = await gitRemoteHandlersV6['git.branch.list']({ cwd: '/repo' })
      expect(result.branches).not.toContain('')
    })
  })

  // ── git.checkout ──────────────────────────────────────────────────────────────
  describe('git.checkout', () => {
    it('checkout branch — trả về { ok: true }', async () => {
      const result = await gitRemoteHandlersV6['git.checkout']({ cwd: '/repo', branch: 'main' })
      expect(result).toEqual({ ok: true })
    })

    it('checkout -b khi create=true', async () => {
      await gitRemoteHandlersV6['git.checkout']({ cwd: '/repo', branch: 'feature/new', create: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['checkout', '-b', 'feature/new'] })
      )
    })

    it('checkout không -b khi create không set', async () => {
      await gitRemoteHandlersV6['git.checkout']({ cwd: '/repo', branch: 'main' })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['checkout', 'main'] })
      )
    })
  })
})
