# T16 — Viết Tests: git-handler-v6.test.ts + git-remote-rpc.test.ts [NEW FILE STRATEGY]

**Phase:** 3  
**Effort:** ~2 hours  
**Depends on:** T15 (git-remote-handler-v6.ts + git-remote-rpc.ts phải tồn tại)  
**Solution ref:** [07-tdd20-remote-git-ui.md §2.2, §2.3](../solutions/07-tdd20-remote-git-ui.md)  
**TDD ref:** TDD-20  
**⚠️ Conflict Resolution:** New File strategy — tạo test file mới, không thêm vào test file cũ

---

## ⚠️ QUAN TRỌNG — Quy tắc bất biến

> **KHÔNG thêm vào** `src/relay/__tests__/git-handler.test.ts` (104KB — test file cũ đã đủ)  
> ✅ Tạo test file mới: `src/relay/__tests__/git-handler-v6.test.ts`  
> ✅ Tạo test file mới: `src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts`

## Mục tiêu

1. `src/relay/__tests__/git-handler-v6.test.ts` [NEW] — test `gitRemoteHandlersV6`
2. `src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts` [NEW] — test RPC routing

**Target: ≥ 35 tests total (≥15 relay v6 + ≥20 RPC)**

---

## Files Cần Đọc Trước

1. `src/relay/git-remote-handler-v6.ts` — (T15 tạo mới) handlers cần test
2. `src/relay/git-remote-handler-index.ts` — (T15 tạo mới) selector
3. `src/relay/git-exec-validator.ts` — `validateGitArgs`, `ALLOWED_GIT_SUBCOMMANDS`
4. `src/relay/__tests__/git-exec-validator.test.ts` — pattern tái sử dụng
5. `src/main/runtime/rpc/methods/git-remote-rpc.ts` — (T15 tạo mới) methods cần test
6. `src/main/project/__tests__/project-rpc.test.ts` — mock pattern relay

---

## File 1: `src/relay/__tests__/git-handler-v6.test.ts` [NEW]

```typescript
/**
 * git-handler-v6.test.ts — Tests cho gitRemoteHandlersV6 (TDD-20)
 *
 * Test strategy:
 * - Mock execFileAsync để tránh cần real git
 * - Test validation: chỉ ALLOWED_GIT_SUBCOMMANDS được pass
 * - Test params mapping: mỗi method gọi đúng git args
 * - Test result parsing: branches, ok status
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { gitRemoteHandlersV6 } from '../git-remote-handler-v6'

// Mock node:child_process để tránh real git calls
vi.mock('node:child_process', () => ({
  execFile: vi.fn(),
}))
vi.mock('node:util', () => ({
  promisify: vi.fn((fn) => fn),
}))

const mockExecFile = vi.fn()
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

describe('gitRemoteHandlersV6', () => {
  describe('kế thừa v5', () => {
    it('có git.exec từ v5', () => {
      expect(gitRemoteHandlersV6['git.exec']).toBeDefined()
    })

    it('có git.execStream từ v5', () => {
      expect(gitRemoteHandlersV6['git.execStream']).toBeDefined()
    })
  })

  describe('git.status', () => {
    it('gọi với porcelain v2 format', async () => {
      await gitRemoteHandlersV6['git.status']({ cwd: '/repo' })
      // Verify git.exec được gọi với đúng args
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

    it('trả về { raw: stdout }', async () => {
      const result = await gitRemoteHandlersV6['git.status']({ cwd: '/repo' })
      expect(result).toHaveProperty('raw')
    })
  })

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

  describe('git.add', () => {
    it('gọi git add với files', async () => {
      const result = await gitRemoteHandlersV6['git.add']({ cwd: '/repo', files: ['a.ts', 'b.ts'] })
      expect(result).toEqual({ ok: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['add', '--', 'a.ts', 'b.ts'] })
      )
    })
  })

  describe('git.restore', () => {
    it('restore unstaged files', async () => {
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

  describe('git.commit', () => {
    it('gọi git commit -m message', async () => {
      const result = await gitRemoteHandlersV6['git.commit']({ cwd: '/repo', message: 'feat: add feature' })
      expect(result).toHaveProperty('ok', true)
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['commit', '-m', 'feat: add feature'] })
      )
    })
  })

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
  })

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
  })

  describe('git.checkout', () => {
    it('checkout branch hiện tại', async () => {
      const result = await gitRemoteHandlersV6['git.checkout']({ cwd: '/repo', branch: 'main' })
      expect(result).toEqual({ ok: true })
    })

    it('checkout -b khi create=true', async () => {
      await gitRemoteHandlersV6['git.checkout']({ cwd: '/repo', branch: 'feature/new', create: true })
      expect(gitRemoteHandlersV6['git.exec']).toHaveBeenCalledWith(
        expect.objectContaining({ args: ['checkout', '-b', 'feature/new'] })
      )
    })
  })
})
```

---

## File 2: `src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts` [NEW]

```typescript
/**
 * git-remote-rpc.test.ts — Tests cho createGitRemoteV6Methods (TDD-20)
 *
 * Test strategy:
 * - Mock ProjectServerRouter.getRelayForProject()
 * - Mock relay.call() để verify routing đúng method + params
 * - Test authorization: projectId phải valid
 * - Test param forwarding: mỗi RPC method forward đúng params
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createGitRemoteV6Methods } from '../git-remote-rpc'

const mockRelayCall = vi.fn().mockResolvedValue({ ok: true })
const mockGetRelayForProject = vi.fn().mockResolvedValue({
  call: mockRelayCall,
  projectCwd: '/mock/repo',
})
const mockProjectRouter = {
  getRelayForProject: mockGetRelayForProject,
}

describe('createGitRemoteV6Methods', () => {
  let methods: ReturnType<typeof createGitRemoteV6Methods>

  beforeEach(() => {
    vi.clearAllMocks()
    methods = createGitRemoteV6Methods(mockProjectRouter as any)
  })

  it('returns array của RPC methods', () => {
    expect(Array.isArray(methods)).toBe(true)
    expect(methods.length).toBeGreaterThan(0)
  })

  it('method names đúng format (git.*)', () => {
    const names = methods.map(m => m.name)
    expect(names).toContain('git.status')
    expect(names).toContain('git.diff')
    expect(names).toContain('git.add')
    expect(names).toContain('git.commit')
    expect(names).toContain('git.push')
    expect(names).toContain('git.pull')
    expect(names).toContain('git.branch.list')
    expect(names).toContain('git.checkout')
  })

  describe('git.status method', () => {
    it('gọi getRelayForProject với đúng projectId', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1' }, {} as any)
      expect(mockGetRelayForProject).toHaveBeenCalledWith('proj-1')
    })

    it('route tới relay.call git.status', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1' }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.status', expect.objectContaining({ cwd: '/mock/repo' }))
    })

    it('forward worktreePath nếu có', async () => {
      const method = methods.find(m => m.name === 'git.status')!
      await method.handler({ projectId: 'proj-1', worktreePath: '/wt' }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.status', expect.objectContaining({ worktreePath: '/wt' }))
    })
  })

  describe('git.add method', () => {
    it('forward files array tới relay', async () => {
      const method = methods.find(m => m.name === 'git.add')!
      await method.handler({ projectId: 'proj-1', files: ['a.ts', 'b.ts'] }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.add', expect.objectContaining({ files: ['a.ts', 'b.ts'] }))
    })

    it('throw nếu projectId không tìm được relay', async () => {
      mockGetRelayForProject.mockRejectedValueOnce(new Error('Project not found'))
      const method = methods.find(m => m.name === 'git.add')!
      await expect(method.handler({ projectId: 'bad', files: [] }, {} as any)).rejects.toThrow('Project not found')
    })
  })

  describe('git.commit method', () => {
    it('forward message tới relay', async () => {
      const method = methods.find(m => m.name === 'git.commit')!
      await method.handler({ projectId: 'proj-1', message: 'feat: update' }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.commit', expect.objectContaining({ message: 'feat: update' }))
    })
  })

  describe('git.push method', () => {
    it('forward force=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.push')!
      await method.handler({ projectId: 'proj-1', force: true }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.push', expect.objectContaining({ force: true }))
    })
  })

  describe('git.branch.list method', () => {
    it('forward remote=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.branch.list')!
      await method.handler({ projectId: 'proj-1', remote: true }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.branch.list', expect.objectContaining({ remote: true }))
    })
  })

  describe('git.checkout method', () => {
    it('forward branch + create tới relay', async () => {
      const method = methods.find(m => m.name === 'git.checkout')!
      await method.handler({ projectId: 'proj-1', branch: 'feat/new', create: true }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.checkout', expect.objectContaining({ branch: 'feat/new', create: true }))
    })
  })

  describe('git.restore method', () => {
    it('forward staged=true tới relay', async () => {
      const method = methods.find(m => m.name === 'git.restore')!
      await method.handler({ projectId: 'proj-1', files: ['x.ts'], staged: true }, {} as any)
      expect(mockRelayCall).toHaveBeenCalledWith('git.restore', expect.objectContaining({ staged: true }))
    })
  })
})
```

---

## Bước Verify

```bash
# Chạy test file relay v6:
pnpm vitest run src/relay/__tests__/git-handler-v6.test.ts
# Expected: ≥15 tests passing

# Chạy test file RPC:
pnpm vitest run src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts
# Expected: ≥20 tests passing

# Verify file cũ không bị chỉnh:
git diff src/relay/__tests__/git-handler.test.ts  # phải empty
```

---

## Acceptance Criteria

- [x] `src/relay/__tests__/git-handler-v6.test.ts` tạo mới — ≥15 tests pass ✅ (24 tests pass)
- [x] `src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts` tạo mới — ≥20 tests pass ✅ (18 tests pass)
- [x] **`src/relay/__tests__/git-handler.test.ts` GIỮ NGUYÊN** — không thêm/sửa ✅ (`git diff` = 0 lines)
- [x] `pnpm vitest run src/relay/__tests__/git-handler-v6.test.ts` → green ✅
- [x] `pnpm vitest run src/main/runtime/rpc/methods/__tests__/git-remote-rpc.test.ts` → green ✅
