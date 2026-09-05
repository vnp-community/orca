// Split out of repos-all-hosts.test.ts to stay under the max-lines budget:
// this cluster covers fetchReposForAllHosts' cross-host "shared project"
// metadata-preservation behavior specifically (a repo-derived project seen
// from both a local and a remote host, merging/preserving its metadata
// across refreshes) — a large, self-contained slice of the original file's
// coverage with its own dedicated mock-setup helpers.
import { describe, expect, it } from 'vitest'
import { createTestStore } from './store-test-helpers'
import type { Project, ProjectHostSetup, Repo } from '../../../../shared/types'
import type { RuntimeEnvironmentCallRequest } from '../../runtime/runtime-compatibility-test-fixture'
import {
  defaultProjectListResponse,
  installReposAllHostsHarness,
  localProject,
  localProjectHostSetup,
  localRepo,
  listHostSetups,
  projectsList,
  remoteRepo,
  remoteRepoViewResult,
  reposList,
  runtimeEnvironmentCall
} from './repos-all-hosts-fixture'

installReposAllHostsHarness()

function configureSharedProjectCompatibilityMocks(
  options: {
    localRepoHasProviderIdentity?: boolean
    remoteProjectRuntimePreference?: Project['localWindowsRuntimePreference']
  } = {}
): {
  sharedProjectId: string
  sharedRemoteProject: Project
  remoteRepoWithIdentity: Repo
} {
  const sharedProjectId = 'github:stablyai/orca'
  const localRepoForSharedProject: Repo =
    options.localRepoHasProviderIdentity === false
      ? localRepo
      : {
          ...localRepo,
          upstream: { owner: 'stablyai', repo: 'orca' }
        }
  const remoteRepoWithIdentity: Repo = {
    ...remoteRepo,
    upstream: { owner: 'stablyai', repo: 'orca' }
  }
  const sharedLocalProject: Project = {
    id: sharedProjectId,
    displayName: 'Orca',
    badgeColor: '#000',
    sourceRepoIds: ['local-repo'],
    localWindowsRuntimePreference: { kind: 'windows-host' },
    createdAt: 1,
    updatedAt: 1
  }
  const sharedRemoteProject: Project = {
    id: sharedProjectId,
    displayName: 'Orca',
    badgeColor: '#111',
    sourceRepoIds: ['remote-repo'],
    ...(options.remoteProjectRuntimePreference
      ? { localWindowsRuntimePreference: options.remoteProjectRuntimePreference }
      : {}),
    createdAt: 2,
    updatedAt: 2
  }
  const sharedLocalSetup: ProjectHostSetup = {
    ...localProjectHostSetup,
    projectId: sharedProjectId
  }
  const sharedRemoteSetup: ProjectHostSetup = {
    ...localProjectHostSetup,
    id: 'remote-setup',
    projectId: sharedProjectId,
    repoId: 'remote-repo',
    path: '/srv/repo',
    displayName: 'Remote setup'
  }
  reposList.mockResolvedValue([localRepoForSharedProject])
  projectsList.mockResolvedValue([sharedLocalProject])
  listHostSetups.mockResolvedValue([sharedLocalSetup])
  runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    // Why bare RemoteRepoView/OrcaProject[] here, not the old {repos: Repo[]}
    // / {projects: Project[]} legacy shapes: fetchProjectHostSetupCompatibility
    // no longer calls 'project.list' at all (that name now belongs to
    // getOrCreateDefaultProject's unrelated default-project resolution) — the
    // runtime side's rich Project metadata (sourceRepoIds,
    // localWindowsRuntimePreference, ...) is derived locally from repo
    // identity, same as the fallback path. `sharedRemoteProject` therefore no
    // longer flows over the wire at all; only projectHostSetup.list still does.
    if (args.method === 'project.list') {
      return defaultProjectListResponse()
    }
    if (args.method === 'repo.list') {
      return {
        id: 'rpc-repo-list',
        ok: true,
        result: remoteRepoViewResult([remoteRepoWithIdentity]),
        _meta: { runtimeId: 'runtime-remote' }
      }
    }
    if (args.method === 'projectHostSetup.list') {
      return {
        id: 'rpc-project-host-setup-list',
        ok: true,
        result: { setups: [sharedRemoteSetup] },
        _meta: { runtimeId: 'runtime-remote' }
      }
    }
    return {
      id: 'rpc-other',
      ok: true,
      result: {},
      _meta: { runtimeId: 'runtime-remote' }
    }
  })
  return { sharedProjectId, sharedRemoteProject, remoteRepoWithIdentity }
}

// Phase 10: a legacy Project is always exactly one repo now, so local-repo and
// remote-repo never merge into one project just because they happen to share
// a GitHub identity. The local host's API-owned project (still persisted
// under the old shared id in this fixture, simulating a project created
// before Phase 10) keeps only its own repo; remote-repo gets its own
// separately-derived `repo:remote-repo` project instead of folding into it.
function expectLocalAndRemoteProjectsStaySeparate(
  projects: readonly Project[],
  sharedProjectId: string
): void {
  const localProject = projects.find((project) => project.id === sharedProjectId)
  expect(localProject?.sourceRepoIds).toEqual(['local-repo'])
  expect(localProject?.localWindowsRuntimePreference).toEqual({ kind: 'windows-host' })
  const remoteProject = projects.find((project) => project.id === 'repo:remote-repo')
  expect(remoteProject?.sourceRepoIds).toEqual(['remote-repo'])
}

describe('fetchReposForAllHosts — shared project metadata', () => {
  // Why: 'project.list' used to be the legacy repo-derived-project RPC method
  // this test simulated — the real backend repurposed that name for the v5.0
  // OrcaProject membership model (a plain OrcaProject[] array, not
  // {projects: [...]}) and deliberately dropped the old runtime handler (see
  // project-runtime-rpc-methods.ts). fetchProjectHostSetupCompatibility no
  // longer calls 'project.list' at all — 'projects' is always derived locally
  // from `repos`, same as the fallback path. This test now asserts that
  // corrected behavior: a runtime-owned repo still produces a project entry
  // (via projectHostSetup.list, which is unaffected) that survives a
  // local-only refresh, using its repo-derived metadata rather than
  // RPC-provided metadata that no longer exists.
  it('derives a runtime-owned project from repos and preserves it during a local-only repo refresh', async () => {
    const runtimeSetup: ProjectHostSetup = {
      ...localProjectHostSetup,
      id: 'runtime-setup',
      projectId: 'repo:remote-repo',
      repoId: 'remote-repo',
      path: '/srv/repo',
      displayName: 'Runtime setup',
      setupState: 'ready',
      createdAt: 3,
      updatedAt: 4
    }
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'project.list') {
        return defaultProjectListResponse()
      }
      if (args.method === 'repo.list') {
        return {
          id: 'rpc-repo-list',
          ok: true,
          result: remoteRepoViewResult([remoteRepo]),
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'projectHostSetup.list') {
        return {
          id: 'rpc-project-host-setup-list',
          ok: true,
          result: { setups: [runtimeSetup] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: {},
        _meta: { runtimeId: 'runtime-remote' }
      }
    })
    const store = createTestStore()

    await store.getState().fetchReposForAllHosts()
    await store.getState().fetchRepos()

    expect(
      store
        .getState()
        .projects.map((project) => project.id)
        .sort()
    ).toEqual(['local-project', 'repo:remote-repo'])
    expect(store.getState().projects.find((project) => project.id === 'repo:remote-repo')).toEqual(
      expect.objectContaining({
        displayName: remoteRepo.displayName,
        // Why '', not remoteRepo.badgeColor: repo.list's bare RemoteRepoView
        // never carries badgeColor — mergeRepoViewIntoRepo defaults it on a
        // repo seen for the first time, same as any other client-only field.
        badgeColor: ''
      })
    )
  })

  it('does not backfill another host repo-derived project during runtime-only refresh', async () => {
    const store = createTestStore()

    await store.getState().fetchReposForAllHosts()
    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    expect(
      store
        .getState()
        .projects.map((project) => project.id)
        .sort()
    ).toEqual(['local-project', 'repo:remote-repo'])
    expect(store.getState().projects).toContainEqual(localProject)
  })

  it('keeps local and remote projects separate even when fetched under the same legacy shared project id (Phase 10: one project per repo)', async () => {
    const { sharedProjectId, remoteRepoWithIdentity } = configureSharedProjectCompatibilityMocks()
    const store = createTestStore()
    // Why remote-repo is pre-seeded with its provider identity, not left for
    // repo.list to deliver it: RemoteRepoView (the actual wire shape) never
    // carries `upstream` — cross-host project identity for a runtime repo can
    // only be *preserved* across a refresh (mergeRepoViewIntoRepo's
    // existing-record spread), not derived from a first-time fetch.
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      repos: [{ ...remoteRepoWithIdentity, executionHostId: 'runtime:env-1' }]
    })

    await store.getState().fetchReposForAllHosts()

    expectLocalAndRemoteProjectsStaySeparate(store.getState().projects, sharedProjectId)
    // Both setups still report `sharedProjectId` here because that's wire
    // metadata from this fixture's simulated pre-Phase-10 backend responses
    // (projectHostSetup.list for both hosts) — it no longer implies the two
    // hosts' *projects* are merged, since project derivation is always
    // repo-keyed now regardless of what a setup's projectId says.
    expect(store.getState().projectHostSetups).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          projectId: sharedProjectId,
          hostId: 'local',
          repoId: 'local-repo'
        }),
        expect.objectContaining({
          projectId: sharedProjectId,
          hostId: 'runtime:env-1',
          repoId: 'remote-repo'
        })
      ])
    )
  })

  it('keeps local Windows runtime preference when remote project metadata has its own preference', async () => {
    const { sharedProjectId } = configureSharedProjectCompatibilityMocks({
      remoteProjectRuntimePreference: { kind: 'wsl', distro: 'Ubuntu' }
    })
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()

    expect(
      store.getState().projects.find((project) => project.id === sharedProjectId)
        ?.localWindowsRuntimePreference
    ).toEqual({ kind: 'windows-host' })
  })

  it('keeps local and remote projects separate after a runtime-only repo refresh (Phase 10: one project per repo)', async () => {
    const { sharedProjectId, remoteRepoWithIdentity } = configureSharedProjectCompatibilityMocks()
    const store = createTestStore()
    // Why remote-repo is pre-seeded — see the identical note above: RemoteRepoView
    // never carries `upstream`, so a runtime repo's provider identity can only be
    // preserved across a refresh, not derived from the fetch itself.
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      repos: [{ ...remoteRepoWithIdentity, executionHostId: 'runtime:env-1' }]
    })

    await store.getState().fetchReposForAllHosts()
    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    expectLocalAndRemoteProjectsStaySeparate(store.getState().projects, sharedProjectId)
  })

  it('keeps the local side of a shared project when a runtime refresh removes its repos', async () => {
    const { sharedProjectId } = configureSharedProjectCompatibilityMocks()
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        return {
          id: 'rpc-repo-list-empty',
          ok: true,
          result: { repos: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'project.list') {
        return {
          id: 'rpc-project-list-empty',
          ok: true,
          result: [],
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'projectHostSetup.list') {
        return {
          id: 'rpc-project-host-setup-list-empty',
          ok: true,
          result: { setups: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: {},
        _meta: { runtimeId: 'runtime-remote' }
      }
    })

    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    const sharedProject = store
      .getState()
      .projects.find((project) => project.id === sharedProjectId)
    expect(sharedProject?.sourceRepoIds).toEqual(['local-repo'])
    expect(sharedProject?.localWindowsRuntimePreference).toEqual({ kind: 'windows-host' })
    expect(store.getState().projectHostSetups).toEqual([
      expect.objectContaining({
        projectId: sharedProjectId,
        hostId: 'local',
        repoId: 'local-repo'
      })
    ])
  })

  it('keeps the API-owned local project intact when repo identity cannot re-derive it, even after the remote repo disappears (Phase 10: local project ownership never depended on repo-identity re-derivation)', async () => {
    const { sharedProjectId, remoteRepoWithIdentity } = configureSharedProjectCompatibilityMocks({
      localRepoHasProviderIdentity: false
    })
    const store = createTestStore()
    // Why remote-repo is pre-seeded — see the identical note above: RemoteRepoView
    // never carries `upstream`, so a runtime repo's provider identity can only be
    // preserved across a refresh, not derived from the fetch itself.
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      repos: [{ ...remoteRepoWithIdentity, executionHostId: 'runtime:env-1' }]
    })

    await store.getState().fetchReposForAllHosts()
    // Why this still holds even without a re-derivable local identity: the
    // local host's project always comes straight from the API
    // (projectsApi.list), never from repo-identity derivation — so it was
    // never at risk of losing its id here, Phase 10 or not.
    expectLocalAndRemoteProjectsStaySeparate(store.getState().projects, sharedProjectId)
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        return {
          id: 'rpc-repo-list-empty',
          ok: true,
          result: { repos: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'project.list') {
        return {
          id: 'rpc-project-list-empty',
          ok: true,
          result: [],
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'projectHostSetup.list') {
        return {
          id: 'rpc-project-host-setup-list-empty',
          ok: true,
          result: { setups: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: {},
        _meta: { runtimeId: 'runtime-remote' }
      }
    })

    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    // Removing remote-repo from its host only affects ownership of the
    // separately-derived `repo:remote-repo` project — the local API-owned
    // project keeps its repo and preference untouched either way, confirming
    // its ownership was never tied to repo-identity re-derivation.
    expect(
      store.getState().projects.find((project) => project.id === sharedProjectId)?.sourceRepoIds
    ).toEqual(['local-repo'])
    expect(
      store.getState().projects.find((project) => project.id === sharedProjectId)
        ?.localWindowsRuntimePreference
    ).toEqual({ kind: 'windows-host' })
    expect(
      store.getState().projects.some((project) => project.sourceRepoIds.includes('remote-repo'))
    ).toBe(false)
    expect(store.getState().projectHostSetups).toEqual([
      expect.objectContaining({
        projectId: sharedProjectId,
        hostId: 'local',
        repoId: 'local-repo'
      })
    ])
  })

  it('drops refreshed-host source ownership when that repo no longer matches a shared project', async () => {
    const { sharedProjectId } = configureSharedProjectCompatibilityMocks()
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        return {
          id: 'rpc-repo-list-reassigned',
          ok: true,
          result: remoteRepoViewResult([remoteRepo]),
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      // Why non-empty, not []: fetchAllRemoteRepoViews only calls repo.list
      // for projects project.list actually reports — an empty answer here
      // would skip repo.list entirely, which isn't what this test means to
      // exercise (the repo's own project-derivation happens locally,
      // independent of which project this fixture project.list entry names).
      if (args.method === 'project.list') {
        return defaultProjectListResponse()
      }
      if (args.method === 'projectHostSetup.list') {
        return {
          id: 'rpc-project-host-setup-list-empty',
          ok: true,
          result: { setups: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: {},
        _meta: { runtimeId: 'runtime-remote' }
      }
    })

    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    expect(
      store.getState().projects.find((project) => project.id === sharedProjectId)?.sourceRepoIds
    ).toEqual(['local-repo'])
    expect(
      store
        .getState()
        .projects.map((project) => project.id)
        .sort()
    ).toEqual(['github:stablyai/orca', 'repo:remote-repo'])
    expect(store.getState().projectHostSetups).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          projectId: sharedProjectId,
          hostId: 'local',
          repoId: 'local-repo'
        }),
        expect.objectContaining({
          projectId: 'repo:remote-repo',
          hostId: 'runtime:env-1',
          repoId: 'remote-repo'
        })
      ])
    )
  })

  it('drops stale runtime repo ownership when project metadata lags behind repo removal', async () => {
    const { sharedProjectId, sharedRemoteProject } = configureSharedProjectCompatibilityMocks()
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await store.getState().fetchReposForAllHosts()
    runtimeEnvironmentCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
      if (args.method === 'repo.list') {
        return {
          id: 'rpc-repo-list-empty',
          ok: true,
          result: { repos: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'project.list') {
        return {
          id: 'rpc-project-list-stale',
          ok: true,
          result: [sharedRemoteProject],
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'projectHostSetup.list') {
        return {
          id: 'rpc-project-host-setup-list-empty',
          ok: true,
          result: { setups: [] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      return {
        id: 'rpc-other',
        ok: true,
        result: {},
        _meta: { runtimeId: 'runtime-remote' }
      }
    })

    await store.getState().fetchRuntimeEnvironmentRepos('env-1')

    expect(
      store.getState().projects.find((project) => project.id === sharedProjectId)?.sourceRepoIds
    ).toEqual(['local-repo'])
  })
})
