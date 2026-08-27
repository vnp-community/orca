import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'

const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })
  vi.stubGlobal('window', {
    api: {
      runtimeEnvironments: { call: runtimeEnvironmentTransportCall }
    }
  })
})

// Why (crash seen live against the 'session-auth' web environment):
// repo.list's handler (backend/src/main/runtime/rpc/methods/repo.ts) always
// returns { repos: Repo[] } — a response missing that field means a
// malformed/dropped RPC payload, not a real empty catalog.
// fetchRuntimeEnvironmentRepos used to let that throw 'Cannot read properties
// of undefined (reading map)' straight out of the RPC layer; it must degrade
// to an empty list instead (store/slices/repos.ts's fetchRepoCatalogForTarget).
describe('fetchRuntimeEnvironmentRepos malformed repo.list response', () => {
  it('treats a repo.list response missing `repos` as an empty catalog instead of throwing', async () => {
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        return { id: 'rpc-repo-list', ok: true, result: {}, _meta: { runtimeId: 'runtime-remote' } }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: { projects: [], setups: [] },
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()

    await expect(store.getState().fetchRuntimeEnvironmentRepos('env-1')).resolves.toEqual([])
    expect(store.getState().repos).toEqual([])
  })
})
