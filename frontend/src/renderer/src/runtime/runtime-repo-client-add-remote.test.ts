// @vitest-environment happy-dom
// addRuntimeRepoRemote must register on the dev server it was given, in the Go repo.add shape
// (SOL-FE-E2E-004); the old {path, kind} + repo.update pair was ignored by the gateway.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  callRuntimeRpc: vi.fn(),
  addRemote: vi.fn()
}))

vi.mock('./runtime-rpc-client', () => ({
  callRuntimeRpc: mocks.callRuntimeRpc,
  getActiveRuntimeTarget: (settings: { activeRuntimeEnvironmentId?: string | null } | null) =>
    settings?.activeRuntimeEnvironmentId
      ? { kind: 'environment', environmentId: settings.activeRuntimeEnvironmentId }
      : { kind: 'local' }
}))

import { resetDefaultProjectCacheForTests } from '@/store/slices/repos'
import { addRuntimeRepoRemote } from './runtime-repo-client'

beforeEach(() => {
  vi.clearAllMocks()
  resetDefaultProjectCacheForTests()
  vi.stubGlobal('window', {
    api: { repos: { addRemote: mocks.addRemote } },
    location: { pathname: '/web-index.html' }
  })
  mocks.callRuntimeRpc.mockImplementation(async (_target: unknown, method: string) => {
    if (method === 'project.list') {
      return [{ id: 'proj-ds1', createdAt: 1, devServerId: 'ds-1' }]
    }
    return { id: 'repo-1', projectId: 'proj-ds1', url: '/srv/app', displayName: 'app', position: 0 }
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('addRuntimeRepoRemote', () => {
  it('registers the path with repo.add carrying the dev server', async () => {
    const result = await addRuntimeRepoRemote(
      { activeRuntimeEnvironmentId: 'env-1' },
      { connectionId: '', remotePath: '/srv/app', kind: 'folder', devServerId: 'ds-1' }
    )

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.add',
      { projectId: 'proj-ds1', url: '/srv/app', displayName: 'app', devServerId: 'ds-1' },
      { timeoutMs: 30_000 }
    )
    expect('repo' in result && result.repo).toMatchObject({ id: 'repo-1', kind: 'folder' })
  })

  it('refuses to register on web without a dev server, before any RPC', async () => {
    await expect(
      addRuntimeRepoRemote(
        { activeRuntimeEnvironmentId: 'env-1' },
        { connectionId: '', remotePath: '/srv/app' }
      )
    ).rejects.toMatchObject({ code: 'REPO_HOST_REQUIRED' })
    expect(mocks.callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('leaves the desktop SSH route alone', async () => {
    mocks.addRemote.mockResolvedValue({ repo: { id: 'ssh-repo' } })
    const args = { connectionId: 'ssh-1', remotePath: '/home/me/app' }

    await expect(addRuntimeRepoRemote(null, args)).resolves.toEqual({ repo: { id: 'ssh-repo' } })

    expect(mocks.addRemote).toHaveBeenCalledWith(args)
    expect(mocks.callRuntimeRpc).not.toHaveBeenCalled()
  })
})
