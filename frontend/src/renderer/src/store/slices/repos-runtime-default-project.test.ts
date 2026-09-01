// Split out of repos.test.ts to stay under the max-lines budget (Phase 4b):
// these 4 tests cover repo.list/repo.add/repo.reorder's shared "resolve an
// implicit default OrcaProject first" behavior (getOrCreateDefaultProject),
// which needs extra project.list mocking every other test in that file
// doesn't.
import { describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import {
  DEFAULT_PROJECT_ID,
  defaultProjectListRuntimeResult,
  installReposRuntimeRoutingHarness,
  localRepo,
  orcaProfileFindProjectProfiles,
  remoteRepo,
  reposAdd,
  reposList,
  reposPickFolder,
  reposReorder,
  runtimeEnvironmentCall
} from './repos-runtime-routing-fixture'

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn()
  }
}))

installReposRuntimeRoutingHarness()

describe('repo slice runtime routing — default project resolution', () => {
  it('fetches repos from the active remote runtime environment', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return defaultProjectListRuntimeResult()
      }
      return {
        id: 'rpc-1',
        ok: true,
        result: {
          repos: [
            {
              id: remoteRepo.id,
              projectId: DEFAULT_PROJECT_ID,
              url: remoteRepo.path,
              displayName: remoteRepo.displayName,
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
      activeRepoId: 'stale-repo',
      filterRepoIds: ['remote-repo', 'stale-repo']
    })

    await store.getState().fetchRepos()

    expect(store.getState().repos).toEqual([
      {
        id: remoteRepo.id,
        projectId: DEFAULT_PROJECT_ID,
        path: remoteRepo.path,
        displayName: remoteRepo.displayName,
        badgeColor: '',
        addedAt: expect.any(Number),
        executionHostId: 'runtime:env-1'
      }
    ])
    expect(store.getState().projects).toEqual([
      expect.objectContaining({ id: 'repo:remote-repo', sourceRepoIds: ['remote-repo'] })
    ])
    expect(store.getState().projectHostSetups).toEqual([
      expect.objectContaining({ id: 'remote-repo', hostId: 'runtime:env-1' })
    ])
    expect(store.getState().activeRepoId).toBeNull()
    expect(store.getState().filterRepoIds).toEqual(['remote-repo'])
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'repo.list',
      params: { projectId: DEFAULT_PROJECT_ID },
      timeoutMs: 15_000
    })
    expect(reposList).not.toHaveBeenCalled()
  })

  it('stamps runtime-fetched SSH repos with the runtime owner', async () => {
    // Why an existing store repo, not a `connectionId` field on the mocked
    // response: RemoteRepoView (repo.list's actual wire shape) never carries
    // connectionId — it only ever appears on a repo already known locally
    // (e.g. added earlier via direct SSH), and stamping must still promote
    // it to the runtime owner once that runtime's own repo.list reports it.
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return defaultProjectListRuntimeResult()
      }
      return {
        id: 'rpc-ssh-repo',
        ok: true,
        result: {
          repos: [
            {
              id: remoteRepo.id,
              projectId: DEFAULT_PROJECT_ID,
              url: remoteRepo.path,
              displayName: remoteRepo.displayName,
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
      repos: [{ ...remoteRepo, connectionId: 'ssh-1', executionHostId: 'runtime:env-1' }]
    })

    await store.getState().fetchRepos()

    expect(store.getState().repos).toEqual([
      {
        ...remoteRepo,
        projectId: DEFAULT_PROJECT_ID,
        connectionId: 'ssh-1',
        executionHostId: 'runtime:env-1'
      }
    ])
  })

  it('adds explicit server paths through the active remote runtime environment', async () => {
    runtimeEnvironmentCall.mockImplementation(async (args: { method: string }) => {
      if (args.method === 'project.list') {
        return defaultProjectListRuntimeResult()
      }
      // Why a bare RepoView, not { repo: ... }: repo.add's Go handler
      // (channels_repo_ssh_status_workspace.go) returns the repoView
      // directly.
      return {
        id: 'rpc-add',
        ok: true,
        result: {
          id: remoteRepo.id,
          projectId: DEFAULT_PROJECT_ID,
          url: remoteRepo.path,
          displayName: remoteRepo.displayName,
          position: 0
        },
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never
    })

    const expectedRepo = {
      id: remoteRepo.id,
      projectId: DEFAULT_PROJECT_ID,
      path: remoteRepo.path,
      displayName: remoteRepo.displayName,
      badgeColor: '',
      addedAt: expect.any(Number),
      kind: 'folder',
      executionHostId: 'runtime:env-1'
    }

    await expect(store.getState().addRepoPath('/srv/project', 'folder')).resolves.toEqual(
      expectedRepo
    )

    expect(store.getState().repos).toEqual([expectedRepo])
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'repo.add',
      params: { projectId: DEFAULT_PROJECT_ID, url: '/srv/project', displayName: 'project' },
      timeoutMs: 15_000
    })
    expect(reposAdd).not.toHaveBeenCalled()
    expect(reposPickFolder).not.toHaveBeenCalled()
    expect(orcaProfileFindProjectProfiles).not.toHaveBeenCalled()
  })

  it('reorders repos through the active remote runtime environment', async () => {
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-4',
      ok: true,
      result: { status: 'applied' },
      _meta: { runtimeId: 'runtime-remote' }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      repos: [localRepo, remoteRepo]
    })

    await store.getState().reorderRepos([remoteRepo.id, localRepo.id])

    expect(store.getState().repos.map((repo) => repo.id)).toEqual([remoteRepo.id, localRepo.id])
    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'repo.reorder',
      // Why projectId: '' — neither fixture repo has a projectId set, and
      // the reorder call site falls back to '' when the reordered repo at
      // the head of this host's group doesn't carry one.
      params: { projectId: '', repoIdsInOrder: [remoteRepo.id, localRepo.id] },
      timeoutMs: 15_000
    })
    expect(reposReorder).not.toHaveBeenCalled()
  })
})
