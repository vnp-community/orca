import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'

// Why: for a web client, fetchReposForAllHosts' `{kind:'local'}` leg (its
// ONLY leg — environments=[] there, see its own isWebClientLocation comment)
// tags a repo with the bare LOCAL_EXECUTION_HOST_ID, while
// fetchRuntimeEnvironmentRepos'/fetchRepos' `{kind:'environment'}` leg tags
// the SAME repo from its devServerId (repoWithFetchedOwner's web branch).
// Both legs compute the SAME merge hostId ('local', per
// getRuntimeTargetHostId's own web-mode override) but, before this fix,
// mergeFetchedReposForHost keyed each repo's identity by its OWN
// (inconsistently tagged) executionHostId instead of that shared hostId —
// so whichever leg ran second saw the other leg's row as "a different
// host's repo" and appended a duplicate instead of updating it in place.
// Regression guard for the live bug this caused (Settings' repo list
// showing every repo twice).
const reposList = vi.fn()
const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

function repoListRpcResponse(devServerId: string): {
  id: string
  ok: true
  result: {
    repos: {
      id: string
      projectId: string
      url: string
      displayName: string
      devServerId: string
      position: number
    }[]
  }
  _meta: { runtimeId: string }
} {
  return {
    id: 'rpc-repo-list',
    ok: true,
    result: {
      repos: [
        {
          id: 'aiops-v3',
          projectId: 'default-project',
          url: '/opt/aiops-v3',
          displayName: 'aiops-v3',
          devServerId,
          position: 0
        }
      ]
    },
    _meta: { runtimeId: 'runtime-remote' }
  }
}

function projectListRpcResponse(): {
  id: string
  ok: true
  result: {
    id: string
    name: string
    defaultBranch: string
    devServerId: string
    visibility: string
    createdAt: number
    updatedAt: number
  }[]
  _meta: { runtimeId: string }
} {
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

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  reposList.mockReset()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()

  // The 'local' leg, for a web client, resolves through window.api.repos.list
  // (installWebPreloadApi's shim) — a bare Repo[], no connectionId/executionHostId.
  reposList.mockResolvedValue([
    {
      id: 'aiops-v3',
      path: '/opt/aiops-v3',
      displayName: 'aiops-v3',
      badgeColor: '#000',
      addedAt: 1,
      devServerId: 'test-01'
    }
  ])
  runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    if (args.method === 'project.list') {
      return projectListRpcResponse()
    }
    if (args.method === 'repo.list') {
      return repoListRpcResponse('test-01')
    }
    return {
      id: 'rpc-other',
      ok: true,
      result: { projects: [], setups: [] },
      _meta: { runtimeId: 'runtime-remote' }
    }
  })
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })

  vi.stubGlobal('window', {
    __ORCA_WEB_CLIENT__: true,
    location: { pathname: '' },
    api: {
      repos: { list: reposList },
      projects: {
        list: vi.fn().mockResolvedValue([]),
        listHostSetups: vi.fn().mockResolvedValue([])
      },
      runtimeEnvironments: {
        call: runtimeEnvironmentTransportCall,
        list: vi.fn().mockResolvedValue([])
      }
    },
    dispatchEvent: vi.fn()
  })
})

describe('web-client repo fetch: local leg vs environment leg', () => {
  it('does not duplicate a repo whose two fetch legs tag it differently', async () => {
    const store = createTestStore()

    // Environment leg runs first (e.g. a Settings-page mount) and tags the
    // row from its devServerId.
    await store.getState().fetchRuntimeEnvironmentRepos('session-auth')
    expect(store.getState().repos.filter((repo) => repo.id === 'aiops-v3')).toHaveLength(1)

    // Local leg runs next (fetchReposForAllHosts' periodic all-host refresh)
    // and — before this fix — computed a different identity for the same row.
    await store.getState().fetchReposForAllHosts()

    const matches = store.getState().repos.filter((repo) => repo.id === 'aiops-v3')
    expect(matches).toHaveLength(1)
    expect(matches[0]?.devServerId).toBe('test-01')
  })

  // Why: fetchReposForAllHosts' {kind:'local'} leg is the FIRST fetch to run
  // on every page load (App.tsx's boot effect) — before this fix, it always
  // hardcoded LOCAL_EXECUTION_HOST_ID for a web client, ignoring devServerId
  // entirely. Regression guard for "Available Hosts showing Local Mac even
  // right after a hard refresh."
  it('resolves executionHostId from devServerId on the very first (local-leg) load', async () => {
    const store = createTestStore()

    await store.getState().fetchReposForAllHosts()

    const repo = store.getState().repos.find((r) => r.id === 'aiops-v3')
    expect(repo?.executionHostId).toBe('devServer:test-01')
  })
})
