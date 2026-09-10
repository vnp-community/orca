import { describe, expect, it } from 'vitest'
import { createTestStore } from './store-test-helpers'
import type { Repo } from '../../../../shared/types'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { getSetupScriptPromptDismissalKey } from '../../lib/setup-script-prompt'
import {
  defaultProjectListResponse,
  installReposAllHostsHarness,
  localFolderWorkspace,
  localProject,
  localProjectGroup,
  localProjectHostSetup,
  localRepo,
  remoteRepo,
  remoteRepoViewResult,
  reposList,
  runtimeEnvironmentCall,
  runtimeEnvironmentsList,
  runtimeEnvironmentTransportCall
} from './repos-all-hosts-fixture'

installReposAllHostsHarness()

function directSshRepo(targetId: string): Repo {
  return {
    ...localRepo,
    id: 're-adopted-repo',
    connectionId: targetId,
    executionHostId: `ssh:${targetId}`
  }
}

const repoReadoption = {
  oldTargetId: 'ssh-old',
  newTargetId: 'ssh-new',
  repoIds: ['re-adopted-repo']
}

describe('fetchReposForAllHosts', () => {
  it('prunes a superseded direct SSH row during a local catalog transaction', async () => {
    const staleRepo = directSshRepo('ssh-old')
    const liveRepo = directSshRepo('ssh-new')
    reposList.mockResolvedValue([liveRepo])
    const store = createTestStore()
    store.setState({
      repos: [staleRepo],
      pendingSshRepoReadoptions: [repoReadoption]
    })

    await store.getState().fetchReposForAllHosts({ remoteHosts: 'skip' })

    expect(store.getState().repos).toEqual([liveRepo])
  })

  it('preserves an old direct SSH row when no re-adoption evidence exists', async () => {
    const staleRepo = directSshRepo('ssh-old')
    const liveRepo = directSshRepo('ssh-new')
    reposList.mockResolvedValue([liveRepo])
    const store = createTestStore()
    store.setState({ repos: [staleRepo] })

    await store.getState().fetchReposForAllHosts({ remoteHosts: 'skip' })

    expect(store.getState().repos).toEqual([staleRepo, liveRepo])
  })

  it('reconciles a targeted runtime response against the latest repo state', async () => {
    const staleRepo = directSshRepo('ssh-old')
    const liveRepo = directSshRepo('ssh-new')
    let resolveRepoList!: (response: unknown) => void
    const repoListResponse = new Promise((resolve) => (resolveRepoList = resolve))
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'project.list') {
        return defaultProjectListResponse()
      }
      if (args.method === 'repo.list') {
        return repoListResponse
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: { projects: [], setups: [] },
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()
    store.setState({
      repos: [staleRepo],
      pendingSshRepoReadoptions: [repoReadoption]
    })

    const load = store.getState().fetchRuntimeEnvironmentRepos('env-1')
    store.setState({ repos: [staleRepo, liveRepo] })
    resolveRepoList({
      id: 'rpc-repo-list',
      ok: true,
      result: remoteRepoViewResult([remoteRepo]),
      _meta: { runtimeId: 'runtime-remote' }
    })
    await load

    expect(store.getState().repos).toEqual([
      liveRepo,
      {
        id: remoteRepo.id,
        projectId: 'default-project',
        path: remoteRepo.path,
        displayName: remoteRepo.displayName,
        badgeColor: '',
        addedAt: expect.any(Number),
        executionHostId: 'runtime:env-1'
      }
    ])
  })

  it('loads local + all configured runtime environments even when a remote env is active', async () => {
    // Why: a cold start that restored a remote workspace leaves the remote
    // environment active. The active-host-only fetchRepos would drop local
    // repos entirely; fetchReposForAllHosts must surface both.
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()

    const ids = store
      .getState()
      .repos.map((repo) => repo.id)
      .sort()
    expect(ids).toEqual(['local-repo', 'remote-repo'])
    expect(store.getState().projects).toContainEqual(localProject)
    expect(store.getState().projectHostSetups).toContainEqual(localProjectHostSetup)
  })

  it('fails soft when a runtime environment is unreachable, keeping local repos', async () => {
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        throw new Error('runtime_unreachable')
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: { projects: [], setups: [] },
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()

    expect(store.getState().repos.map((repo) => repo.id)).toEqual(['local-repo'])
  })

  it('can load only the local catalog slice for first-paint startup', async () => {
    const store = createTestStore()

    await store.getState().fetchReposForAllHosts({ remoteHosts: 'skip' })
    await store.getState().fetchProjectGroupsForAllHosts({ remoteHosts: 'skip' })
    await store.getState().fetchFolderWorkspacesForAllHosts({ remoteHosts: 'skip' })

    expect(runtimeEnvironmentsList).not.toHaveBeenCalled()
    expect(runtimeEnvironmentTransportCall).not.toHaveBeenCalled()
    expect(store.getState().repos).toEqual([{ ...localRepo, executionHostId: 'local' }])
    expect(store.getState().projectGroups).toEqual([
      { ...localProjectGroup, executionHostId: 'local' }
    ])
    expect(store.getState().folderWorkspaces).toEqual([localFolderWorkspace])
  })

  it('preserves remote repo filters during first-paint local catalog refresh', async () => {
    const store = createTestStore()
    const remoteDismissalKey = getSetupScriptPromptDismissalKey('remote-repo')
    const staleDismissalKey = getSetupScriptPromptDismissalKey('stale-repo')
    store.setState({
      activeRepoId: 'remote-repo',
      filterRepoIds: ['remote-repo', 'stale-repo'],
      setupScriptPromptDismissedRepoIds: [remoteDismissalKey, staleDismissalKey],
      trustedOrcaHooks: {
        'remote-repo': { all: { approvedAt: 1 } },
        'stale-repo': { all: { approvedAt: 2 } }
      }
    })

    await store.getState().fetchReposForAllHosts({ remoteHosts: 'skip' })

    expect(store.getState().activeRepoId).toBe('remote-repo')
    expect(store.getState().filterRepoIds).toEqual(['remote-repo', 'stale-repo'])
    expect(store.getState().setupScriptPromptDismissedRepoIds).toEqual([
      remoteDismissalKey,
      staleDismissalKey
    ])
    expect(store.getState().trustedOrcaHooks).toEqual({
      'remote-repo': { all: { approvedAt: 1 } },
      'stale-repo': { all: { approvedAt: 2 } }
    })

    await store.getState().fetchReposForAllHosts()

    expect(store.getState().activeRepoId).toBe('remote-repo')
    expect(store.getState().filterRepoIds).toEqual(['remote-repo'])
    expect(store.getState().setupScriptPromptDismissedRepoIds).toEqual([remoteDismissalKey])
    expect(store.getState().trustedOrcaHooks).toEqual({
      'remote-repo': { all: { approvedAt: 1 } }
    })
  })

  it('starts remote repo catalog loads concurrently for all configured runtimes', async () => {
    runtimeEnvironmentsList.mockResolvedValue([
      { id: 'env-1', name: 'first' },
      { id: 'env-2', name: 'second' }
    ])
    const firstStatusResolvers = new Map<string, (value: unknown) => void>()
    let resolveBothStatusProbes = (): void => {}
    const bothStatusProbesStarted = new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(new Error('Timed out waiting for both runtime probes')),
        1_000
      )
      resolveBothStatusProbes = () => {
        clearTimeout(timeout)
        resolve()
      }
    })
    runtimeEnvironmentTransportCall.mockImplementation(
      (args: RuntimeEnvironmentCallRequest & { selector?: string }) => {
        if (
          args.method === 'status.get' &&
          args.selector &&
          !firstStatusResolvers.has(args.selector)
        ) {
          return new Promise((resolve) => {
            firstStatusResolvers.set(args.selector!, resolve)
            if (firstStatusResolvers.size === 2) {
              resolveBothStatusProbes()
            }
          })
        }
        return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
      }
    )
    const store = createTestStore()

    const load = store.getState().fetchReposForAllHosts()
    await bothStatusProbesStarted

    expect([...firstStatusResolvers.keys()].sort()).toEqual(['env-1', 'env-2'])
    for (const resolve of firstStatusResolvers.values()) {
      resolve(createCompatibleRuntimeStatusResponseIfNeeded({ method: 'status.get' }))
    }
    await load

    expect(
      store
        .getState()
        .repos.map((repo) => `${repo.id}:${repo.executionHostId}`)
        .sort()
    ).toEqual(['local-repo:local', 'remote-repo:runtime:env-1', 'remote-repo:runtime:env-2'])
  })
})
