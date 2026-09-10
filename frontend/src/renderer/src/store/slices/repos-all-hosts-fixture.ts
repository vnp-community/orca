import { beforeEach, vi, type Mock } from 'vitest'
import type {
  FolderWorkspace,
  Project,
  ProjectHostSetup,
  ProjectGroup,
  Repo
} from '../../../../shared/types'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'
import { resetDefaultProjectCacheForTests } from './repos'

// Shared harness for the fetchReposForAllHosts suite (split across
// repos-all-hosts.test.ts and repos-all-hosts-shared-project-metadata.test.ts
// to keep each file under the max-lines budget): sample repos/projects, the
// IPC/runtime mocks, and the window stub reset between tests.

// Phase 4b: repo.list (and repo.add/create/clone) all resolve an implicit
// default OrcaProject first (getOrCreateDefaultProject -> project.list) — a
// bare OrcaProject[], never the legacy {projects: [...]} shape. This fixed
// stub keeps that resolution deterministic across this file's many
// runtimeEnvironmentCall overrides without every one of them needing to
// simulate project.create too.
export function defaultProjectListResponse(): {
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

export function remoteRepoViewResult(repos: readonly Repo[]): {
  repos: { id: string; projectId: string; url: string; displayName: string; position: number }[]
} {
  return {
    repos: repos.map((repo, index) => ({
      id: repo.id,
      projectId: 'default-project',
      url: repo.path,
      displayName: repo.displayName,
      position: index
    }))
  }
}

export const localRepo: Repo = {
  id: 'local-repo',
  path: '/local',
  displayName: 'Local',
  badgeColor: '#000',
  addedAt: 1
}

export const remoteRepo: Repo = {
  id: 'remote-repo',
  path: '/srv/repo',
  displayName: 'Remote',
  badgeColor: '#000',
  addedAt: 1
}

export const localProject: Project = {
  id: 'local-project',
  displayName: 'Local project',
  badgeColor: '#000',
  sourceRepoIds: ['local-repo'],
  createdAt: 1,
  updatedAt: 1
}

export const localProjectHostSetup: ProjectHostSetup = {
  id: 'local-setup',
  projectId: 'local-project',
  hostId: 'local',
  repoId: 'local-repo',
  path: '/local',
  displayName: 'Local setup',
  setupState: 'setting-up',
  setupMethod: 'imported-existing-folder',
  createdAt: 1,
  updatedAt: 1
}

export const localProjectGroup: ProjectGroup = {
  id: 'local-group',
  name: 'Local group',
  parentPath: '/local',
  parentGroupId: null,
  createdFrom: 'manual',
  tabOrder: 0,
  isCollapsed: false,
  color: null,
  createdAt: 1,
  updatedAt: 1
}

export const remoteProjectGroup: ProjectGroup = {
  id: 'remote-group',
  name: 'Remote group',
  parentPath: '/srv',
  parentGroupId: null,
  createdFrom: 'manual',
  tabOrder: 0,
  isCollapsed: false,
  color: null,
  createdAt: 1,
  updatedAt: 1
}

export const localFolderWorkspace: FolderWorkspace = {
  id: 'local-folder',
  projectGroupId: 'local-group',
  name: 'Local folder',
  folderPath: '/local',
  linkedTask: null,
  comment: '',
  isArchived: false,
  isUnread: false,
  isPinned: false,
  sortOrder: 0,
  lastActivityAt: 1,
  createdAt: 1,
  updatedAt: 1
}

export const remoteFolderWorkspace: FolderWorkspace = {
  id: 'remote-folder',
  projectGroupId: 'remote-group',
  name: 'Remote folder',
  folderPath: '/srv',
  linkedTask: null,
  comment: '',
  isArchived: false,
  isUnread: false,
  isPinned: false,
  sortOrder: 0,
  lastActivityAt: 1,
  createdAt: 1,
  updatedAt: 1
}

export const reposList: Mock = vi.fn()
export const projectsList: Mock = vi.fn()
export const listHostSetups: Mock = vi.fn()
export const projectGroupsList: Mock = vi.fn()
export const folderWorkspacesList: Mock = vi.fn()
export const runtimeEnvironmentsList: Mock = vi.fn()
export const runtimeEnvironmentCall: Mock = vi.fn()
export const runtimeEnvironmentTransportCall: Mock = vi.fn()
export const dispatchEventMock: Mock = vi.fn()

// Registers the per-test reset + window stub. Call once inside the suite's module scope.
export function installReposAllHostsHarness(): void {
  beforeEach(() => {
    clearRuntimeCompatibilityCacheForTests()
    resetDefaultProjectCacheForTests()
    reposList.mockReset()
    projectsList.mockReset()
    listHostSetups.mockReset()
    projectGroupsList.mockReset()
    folderWorkspacesList.mockReset()
    runtimeEnvironmentsList.mockReset()
    runtimeEnvironmentCall.mockReset()
    runtimeEnvironmentTransportCall.mockReset()
    dispatchEventMock.mockReset()

    reposList.mockResolvedValue([localRepo])
    projectsList.mockResolvedValue([localProject])
    listHostSetups.mockResolvedValue([localProjectHostSetup])
    projectGroupsList.mockResolvedValue([localProjectGroup])
    folderWorkspacesList.mockResolvedValue([localFolderWorkspace])
    runtimeEnvironmentsList.mockResolvedValue([{ id: 'env-1', name: 'lobster' }])
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
      if (args.method === 'projectGroup.list') {
        return {
          id: 'rpc-project-group-list',
          ok: true,
          result: { groups: [remoteProjectGroup] },
          _meta: { runtimeId: 'runtime-remote' }
        }
      }
      if (args.method === 'folderWorkspace.list') {
        return {
          id: 'rpc-folder-workspace-list',
          ok: true,
          result: { folderWorkspaces: [remoteFolderWorkspace] },
          _meta: { runtimeId: 'runtime-remote' }
        }
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
      api: {
        repos: { list: reposList },
        projects: { list: projectsList, listHostSetups: listHostSetups },
        projectGroups: { list: projectGroupsList },
        folderWorkspaces: { list: folderWorkspacesList },
        runtimeEnvironments: {
          call: runtimeEnvironmentTransportCall,
          list: runtimeEnvironmentsList
        }
      },
      // Why: isWebClientLocation() (used by the fetch*ForAllHosts actions to skip
      // the redundant environment-loop fetch for paired web clients) reads
      // window.location.pathname — a bare stub without it throws.
      location: { pathname: '' },
      dispatchEvent: dispatchEventMock
    })
  })
}
