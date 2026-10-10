import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  callRuntimeRpc: vi.fn(),
  toastError: vi.fn(),
  repos: [] as { id: string; path: string; devServerId?: string | null }[]
}))

vi.mock('sonner', () => ({ toast: { error: mocks.toastError } }))
vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: mocks.callRuntimeRpc,
  getActiveRuntimeTarget: () => ({ kind: 'environment', environmentId: 'env-1' })
}))
vi.mock('@/lib/worktree-creation-flow', () => ({ retryBackgroundWorktreeCreation: vi.fn() }))
vi.mock('@/store', () => ({
  useAppStore: { getState: () => ({ settings: {}, repos: mocks.repos }) }
}))

import { initializeRepoAsGit } from './init-repo-as-git'

describe('initializeRepoAsGit', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.callRuntimeRpc.mockResolvedValue({ path: '/srv/app', defaultBranch: 'main' })
    mocks.repos = [
      { id: 'bound', path: '/srv/app', devServerId: 'ds-1' },
      { id: 'unbound', path: '/srv/old', devServerId: '' }
    ]
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('runs git init on the repo’s own dev server', async () => {
    await expect(initializeRepoAsGit('bound', {})).resolves.toBeNull()

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.create',
      expect.objectContaining({ devServerId: 'ds-1', destPath: '/srv/app' }),
      { timeoutMs: 30_000 }
    )
  })

  it('on web, explains that a repo without a dev server needs one instead of sending an empty host', async () => {
    vi.stubGlobal('window', { location: { pathname: '/web-index.html' } })

    const failure = await initializeRepoAsGit('unbound', {})

    expect(failure).toBe('Choose a dev server for this project first.')
    expect(mocks.toastError).toHaveBeenCalledWith(failure)
    expect(mocks.callRuntimeRpc).not.toHaveBeenCalled()
  })
})
