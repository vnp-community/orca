// @vitest-environment happy-dom
// listRuntimeRepoBranches (repo.listBranches, CONTRACT 2.12): a gateway or host that cannot list says "unsupported" so the
// picker searches; any other failure stays an error.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ callRuntimeRpc: vi.fn(), listBranches: vi.fn() }))

vi.mock('./runtime-rpc-client', () => ({
  callRuntimeRpc: mocks.callRuntimeRpc,
  getActiveRuntimeTarget: (settings: { activeRuntimeEnvironmentId?: string | null } | null) =>
    settings?.activeRuntimeEnvironmentId
      ? { kind: 'environment', environmentId: settings.activeRuntimeEnvironmentId }
      : { kind: 'local' }
}))

import { listRuntimeRepoBranches } from './runtime-repo-client'

const env = { activeRuntimeEnvironmentId: 'env-1' }
const rpcError = (code: string, message: string): Error & { code: string } =>
  Object.assign(new Error(message), { code })

let repos: Record<string, unknown>

beforeEach(() => {
  vi.clearAllMocks()
  repos = {}
  vi.stubGlobal('window', {
    api: {
      get repos() {
        return repos
      }
    }
  })
})
afterEach(() => {
  vi.unstubAllGlobals()
})

describe('listRuntimeRepoBranches', () => {
  it('asks the gateway for the repo and returns the branches with the truncation flag', async () => {
    mocks.callRuntimeRpc.mockResolvedValue({
      branches: [
        { name: 'main', remote: false },
        { name: 'origin/hotfix/x', remote: true }
      ],
      truncated: true
    })
    await expect(listRuntimeRepoBranches(env, 'repo-1')).resolves.toEqual({
      status: 'ok',
      branches: [
        { name: 'main', remote: false },
        { name: 'origin/hotfix/x', remote: true }
      ],
      truncated: true
    })
    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.listBranches',
      { repoId: 'repo-1' },
      { timeoutMs: 15_000 }
    )
  })

  it('tolerates a result with no branches or flag', async () => {
    mocks.callRuntimeRpc.mockResolvedValue({})
    await expect(listRuntimeRepoBranches(env, 'repo-1')).resolves.toEqual({
      status: 'ok',
      branches: [],
      truncated: false
    })
  })

  it.each([
    ['method_not_found code', rpcError('method_not_found', 'Unknown method')],
    [
      'the gateway answer for an unregistered channel',
      rpcError('internal', 'channel "repo.listBranches" is not yet implemented in backend-go')
    ],
    [
      'a web preload error that lost its code',
      new Error('channel "repo.listBranches" is not yet implemented in backend-go')
    ]
  ])('reports unsupported for %s', async (_name, error) => {
    mocks.callRuntimeRpc.mockRejectedValue(error)
    await expect(listRuntimeRepoBranches(env, 'repo-1')).resolves.toEqual({ status: 'unsupported' })
  })

  it.each([
    ['a missing repo', rpcError('internal', 'WORKTREE_REPO_NOT_FOUND: repo not found')],
    ['an unreachable host', rpcError('runtime_unavailable', 'host unreachable')]
  ])('keeps %s an error', async (_name, error) => {
    mocks.callRuntimeRpc.mockRejectedValue(error)
    await expect(listRuntimeRepoBranches(env, 'repo-1')).rejects.toBe(error)
  })

  it('is unsupported on the local path when the preload has no listing, without inventing IPC', async () => {
    await expect(listRuntimeRepoBranches(null, 'repo-1')).resolves.toEqual({
      status: 'unsupported'
    })
    expect(mocks.callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('uses the local preload listing when it exists', async () => {
    repos = { listBranches: mocks.listBranches }
    mocks.listBranches.mockResolvedValue({
      branches: [{ name: 'main', remote: false }],
      truncated: false
    })
    await expect(listRuntimeRepoBranches(null, 'repo-1')).resolves.toEqual({
      status: 'ok',
      branches: [{ name: 'main', remote: false }],
      truncated: false
    })
    expect(mocks.listBranches).toHaveBeenCalledWith({ repoId: 'repo-1' })
  })
})
