import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { RuntimeRpcResponse } from '../../../shared/runtime-rpc-envelope'

// Why this file exists: "Open in Git" on a task worktree showed an empty Source Control list
// (TASK-V2-027 / TASK-FE-E2E-014) because ONE offline repo failed worktree.detectedList and the
// path resolver then claimed no worktree owned the task's checkout.

class MemoryStorage implements Storage {
  private readonly values = new Map<string, string>()
  get length(): number {
    return this.values.size
  }
  clear(): void {
    this.values.clear()
  }
  getItem(key: string): string | null {
    return this.values.get(key) ?? null
  }
  key(index: number): string | null {
    return Array.from(this.values.keys())[index] ?? null
  }
  removeItem(key: string): void {
    this.values.delete(key)
  }
  setItem(key: string, value: string): void {
    this.values.set(key, value)
  }
}

type Handler = (params: unknown) => unknown | Error

function worktreeRow(id: string, repoId: string, path: string): Record<string, unknown> {
  return {
    id,
    repoId,
    path,
    head: 'abc123',
    branch: 'refs/heads/task/1',
    isBare: false,
    isMainWorktree: false,
    displayName: id,
    comment: '',
    linkedIssue: null,
    linkedPR: null,
    linkedLinearIssue: null,
    linkedGitLabMR: null,
    linkedGitLabIssue: null,
    isArchived: false,
    isUnread: false,
    isPinned: false,
    sortOrder: 0,
    lastActivityAt: 0,
    workspaceStatus: 'todo'
  }
}

async function installWithRuntime(handlers: Record<string, Handler>): Promise<{
  status: (path: string) => Promise<unknown>
  calls: { method: string; params: unknown }[]
}> {
  const calls: { method: string; params: unknown }[] = []
  vi.doMock('./web-runtime-client', () => ({
    WebRuntimeClient: class {
      call(method: string, params?: unknown): Promise<RuntimeRpcResponse<unknown>> {
        calls.push({ method, params })
        const handler = handlers[method]
        const meta = { runtimeId: 'runtime-1' }
        if (!handler) {
          return Promise.resolve({
            id: `c${calls.length}`,
            ok: false,
            error: { code: 'unexpected_method', message: `Unexpected method: ${method}` },
            _meta: meta
          })
        }
        const out = handler(params)
        if (out instanceof Error) {
          return Promise.resolve({
            id: `c${calls.length}`,
            ok: false,
            error: { code: 'internal', message: out.message },
            _meta: meta
          })
        }
        return Promise.resolve({ id: `c${calls.length}`, ok: true, result: out, _meta: meta })
      }
      close(): void {}
    }
  }))
  const storage = new MemoryStorage()
  const windowStub = {
    localStorage: storage,
    location: { protocol: 'http:', reload: vi.fn() },
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    atob: (v: string) => Buffer.from(v, 'base64').toString('binary'),
    btoa: (v: string) => Buffer.from(v, 'binary').toString('base64')
  } as unknown as Window & typeof globalThis
  vi.stubGlobal('window', windowStub)
  vi.stubGlobal('navigator', { userAgent: 'Linux', hardwareConcurrency: 8 })
  const { wrapDeviceToken } = await import('./web-runtime-environment-crypto')
  storage.setItem(
    'orca.web.runtimeEnvironment.v1',
    JSON.stringify({
      id: 'web-env-1',
      name: 'Test runtime',
      createdAt: 1,
      updatedAt: 1,
      lastUsedAt: null,
      runtimeId: null,
      preferredEndpointId: 'ws-web-env-1',
      endpoints: [
        {
          id: 'ws-web-env-1',
          kind: 'websocket',
          label: 'WebSocket',
          endpoint: 'ws://127.0.0.1:1234',
          deviceToken: wrapDeviceToken('token'),
          publicKeyB64: 'public-key'
        }
      ]
    })
  )
  const { installWebPreloadApi } = await import('./web-preload-api')
  installWebPreloadApi()
  return {
    status: (path) => windowStub.api.git.status({ worktreePath: path }),
    calls
  }
}

const TASK_PATH = '/private/var/folders/x/e2e-repo-task-1'

describe('web git.status on a task worktree', () => {
  beforeEach(() => {
    vi.resetModules()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.doUnmock('./web-runtime-client')
    vi.restoreAllMocks()
  })

  it('resolves the task worktree even when another project repo fails worktree.detectedList', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { status, calls } = await installWithRuntime({
      'project.list': () => [
        { id: 'proj-offline', createdAt: 1 },
        { id: 'proj-task', createdAt: 2 }
      ],
      'repo.list': (p) => ({
        repos: [
          (p as { projectId: string }).projectId === 'proj-offline'
            ? {
                id: 'repo-offline',
                projectId: 'proj-offline',
                url: '/gone/repo',
                displayName: 'x',
                position: 0
              }
            : {
                id: 'repo-task',
                projectId: 'proj-task',
                url: '/private/var/folders/x/e2e-repo',
                displayName: 'y',
                position: 0
              }
        ]
      }),
      'worktree.detectedList': (p) =>
        (p as { repoId: string }).repoId === 'repo-offline'
          ? new Error('WORKTREE_DETECT_FAILED: git worktree list failed')
          : {
              repoId: 'repo-task',
              authoritative: true,
              worktrees: [worktreeRow('wt-task', 'repo-task', TASK_PATH)]
            },
      'git.status': () => ({ entries: [{ path: 'src/calc.py' }], conflictOperation: 'unknown' })
    })

    await expect(status(TASK_PATH)).resolves.toMatchObject({ entries: [{ path: 'src/calc.py' }] })
    expect(calls.find((c) => c.method === 'git.status')?.params).toMatchObject({
      worktree: 'id:wt-task'
    })
  })

  it('falls back to the bookkept row when git lists the checkout under its symlink-resolved path', async () => {
    const symlinkedPath = '/var/folders/x/e2e-repo-task-1'
    const { status, calls } = await installWithRuntime({
      'project.list': () => [{ id: 'proj-task', createdAt: 1 }],
      'repo.list': () => ({
        repos: [
          {
            id: 'repo-task',
            projectId: 'proj-task',
            url: '/var/folders/x/e2e-repo',
            displayName: 'y',
            position: 0
          }
        ]
      }),
      'worktree.detectedList': () => ({
        repoId: 'repo-task',
        authoritative: true,
        worktrees: [
          worktreeRow('repoid::/private/var/folders/x/e2e-repo-task-1', 'repo-task', TASK_PATH)
        ]
      }),
      'worktree.list': () => ({ worktrees: [worktreeRow('wt-task', 'repo-task', symlinkedPath)] }),
      'git.status': () => ({ entries: [], conflictOperation: 'unknown' })
    })

    await status(symlinkedPath)
    expect(calls.find((c) => c.method === 'git.status')?.params).toMatchObject({
      worktree: 'id:wt-task'
    })
    expect(calls.find((c) => c.method === 'worktree.list')?.params).toMatchObject({
      projectId: 'proj-task'
    })
  })

  it('still reports an unowned path', async () => {
    const { status } = await installWithRuntime({
      'project.list': () => [{ id: 'proj-task', createdAt: 1 }],
      'repo.list': () => ({ repos: [] }),
      'worktree.list': () => ({ worktrees: [] })
    })
    await expect(status('/nowhere/else')).rejects.toThrow('No runtime worktree owns /nowhere/else')
  })
})
