import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Repo } from '../../../../shared/types'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'
import { createTestStore } from './store-test-helpers'
import { resetDefaultProjectCacheForTests } from './repos'

const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  resetDefaultProjectCacheForTests()
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

describe('repo slice runtime project groups', () => {
  it('keeps runtime copies of a grouped canonical project in the same project group', async () => {
    const gitRemoteIdentity = {
      canonicalKey: 'github.com/stablyai/orca',
      remoteName: 'origin',
      remoteUrl: 'https://github.com/stablyai/orca.git'
    }
    const localOrca: Repo = {
      id: 'local-orca',
      path: '/Users/alice/stably/orca',
      displayName: 'orca',
      badgeColor: '#000',
      addedAt: 1,
      executionHostId: 'local',
      gitRemoteIdentity,
      projectGroupId: 'group-orca'
    }
    const runtimeOrca: Repo = {
      id: 'runtime-orca',
      path: '/vercel/sandbox/orca',
      displayName: 'orca',
      badgeColor: '#111',
      addedAt: 2,
      gitRemoteIdentity
    }
    // Why the runtime repo is pre-seeded, not left for first-sight
    // inheritance: repo.list's actual wire shape (RemoteRepoView) never
    // carries gitRemoteIdentity, so a runtime repo's project-group
    // membership can only be *preserved* across a refresh (via
    // mergeRepoViewIntoRepo's existing-record spread) — a brand new
    // runtime-fetched repo has no cross-host identity to inherit through.
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return {
          id: 'rpc-project-list',
          ok: true,
          result: [
            {
              id: 'default-project',
              name: 'My Repos',
              defaultBranch: 'main',
              devServerId: '',
              visibility: 'private',
              createdAt: 1,
              updatedAt: 1
            }
          ],
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-runtime-orca',
        ok: true,
        result: {
          repos: [
            {
              id: runtimeOrca.id,
              projectId: 'default-project',
              url: runtimeOrca.path,
              displayName: runtimeOrca.displayName,
              position: 0
            }
          ]
        },
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      repos: [
        localOrca,
        { ...runtimeOrca, executionHostId: 'runtime:env-1', projectGroupId: 'group-orca' }
      ]
    })

    await store.getState().fetchRepos()

    expect(store.getState().repos).toEqual([
      localOrca,
      {
        ...runtimeOrca,
        executionHostId: 'runtime:env-1',
        projectGroupId: 'group-orca',
        projectId: 'default-project'
      }
    ])
  })
})
