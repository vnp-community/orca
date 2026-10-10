import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  callRuntimeRpc: vi.fn(),
  fetchRepos: vi.fn(),
  fetchWorktrees: vi.fn()
}))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: mocks.callRuntimeRpc,
  getActiveRuntimeTarget: () => ({ kind: 'environment', environmentId: 'env-1' })
}))

vi.mock('@/store', () => ({
  useAppStore: {
    getState: () => ({
      settings: {},
      fetchRepos: mocks.fetchRepos,
      fetchWorktrees: mocks.fetchWorktrees
    })
  }
}))

import { rebindRepoToDevServer } from './repo-host-rebind'

describe('rebindRepoToDevServer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.fetchRepos.mockResolvedValue(undefined)
    mocks.fetchWorktrees.mockResolvedValue(undefined)
  })

  it('calls repo.rebindDevServer then refreshes repos and that repo’s worktrees', async () => {
    mocks.callRuntimeRpc.mockResolvedValue({})

    await expect(rebindRepoToDevServer('repo-1', 'ds-2')).resolves.toEqual({ ok: true })

    expect(mocks.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'environment', environmentId: 'env-1' },
      'repo.rebindDevServer',
      { repoId: 'repo-1', newDevServerId: 'ds-2' }
    )
    expect(mocks.fetchRepos).toHaveBeenCalled()
    expect(mocks.fetchWorktrees).toHaveBeenCalledWith('repo-1')
  })

  it('still reports success when the follow-up refresh fails', async () => {
    mocks.callRuntimeRpc.mockResolvedValue({})
    mocks.fetchRepos.mockRejectedValue(new Error('network'))
    vi.spyOn(console, 'warn').mockImplementation(() => {})

    await expect(rebindRepoToDevServer('repo-1', 'ds-2')).resolves.toEqual({ ok: true })
  })

  it.each([
    ['PROJECT_HAS_ACTIVE_WORKFLOWS: 2 running', 'active-workflows', 'Stop the tasks running'],
    ['rpc error: PROJECT_DEV_SERVER_NOT_FOUND', 'dev-server-not-found', 'no longer exists'],
    ['FORBIDDEN: not an owner', 'forbidden', 'do not have permission'],
    ['something else broke', 'failed', 'something else broke']
  ])('maps "%s" to %s', async (raw, kind, text) => {
    mocks.callRuntimeRpc.mockRejectedValue(new Error(raw))

    const result = await rebindRepoToDevServer('repo-1', 'ds-2')

    expect(result).toMatchObject({ ok: false, kind })
    expect(result.ok === false && result.message).toContain(text)
    expect(mocks.fetchRepos).not.toHaveBeenCalled()
  })
})
