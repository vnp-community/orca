import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import type { NestedRepoScanResult, Repo, ProjectGroup } from '../../../../shared/types'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'

const remoteRepo: Repo = {
  id: 'remote-repo',
  path: '/remote',
  displayName: 'Remote',
  badgeColor: '#111',
  addedAt: 2
}

const projectGroup: ProjectGroup = {
  id: 'group-1',
  name: 'Platform',
  parentPath: null,
  parentGroupId: null,
  createdFrom: 'manual',
  tabOrder: 0,
  isCollapsed: false,
  color: null,
  createdAt: 1,
  updatedAt: 1
}

const reposList = vi.fn()
const reposRemove = vi.fn()
const ptyKill = vi.fn()
const projectGroupsList = vi.fn()
const projectGroupsCreate = vi.fn()
const projectGroupsDelete = vi.fn()
const projectGroupsMoveProject = vi.fn()
const projectGroupsImportNested = vi.fn()
const projectGroupsScanNested = vi.fn()
const projectGroupsCancelNestedScan = vi.fn()
const projectGroupsOnNestedScanProgress = vi.fn()
const folderWorkspacesList = vi.fn()
const folderWorkspacesGetPathStatus = vi.fn()
const folderWorkspacesCreate = vi.fn()
const folderWorkspacesUpdate = vi.fn()
const folderWorkspacesDelete = vi.fn()
const runtimeEnvironmentCall = vi.fn()
const runtimeEnvironmentTransportCall = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  reposList.mockReset()
  reposRemove.mockReset()
  reposRemove.mockResolvedValue(undefined)
  ptyKill.mockReset()
  projectGroupsList.mockReset()
  projectGroupsCreate.mockReset()
  projectGroupsDelete.mockReset()
  projectGroupsMoveProject.mockReset()
  projectGroupsImportNested.mockReset()
  projectGroupsScanNested.mockReset()
  projectGroupsCancelNestedScan.mockReset()
  projectGroupsOnNestedScanProgress.mockReset()
  projectGroupsOnNestedScanProgress.mockReturnValue(vi.fn())
  folderWorkspacesList.mockReset()
  folderWorkspacesGetPathStatus.mockReset()
  folderWorkspacesGetPathStatus.mockResolvedValue({ path: '/workspace/platform', exists: true })
  folderWorkspacesCreate.mockReset()
  folderWorkspacesUpdate.mockReset()
  folderWorkspacesDelete.mockReset()
  runtimeEnvironmentCall.mockReset()
  runtimeEnvironmentTransportCall.mockReset()
  runtimeEnvironmentTransportCall.mockImplementation((args: RuntimeEnvironmentCallRequest) => {
    return createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
  })
  vi.stubGlobal('window', {
    api: {
      repos: {
        list: reposList,
        remove: reposRemove
      },
      pty: { kill: ptyKill },
      projectGroups: {
        list: projectGroupsList,
        create: projectGroupsCreate,
        delete: projectGroupsDelete,
        moveProject: projectGroupsMoveProject,
        scanNested: projectGroupsScanNested,
        cancelNestedScan: projectGroupsCancelNestedScan,
        onNestedScanProgress: projectGroupsOnNestedScanProgress,
        importNested: projectGroupsImportNested
      },
      folderWorkspaces: {
        list: folderWorkspacesList,
        getPathStatus: folderWorkspacesGetPathStatus,
        create: folderWorkspacesCreate,
        update: folderWorkspacesUpdate,
        delete: folderWorkspacesDelete
      },
      runtimeEnvironments: { call: runtimeEnvironmentTransportCall }
    }
  })
})

describe('project group store routing — nested import & group moves', () => {
  it('refreshes local repos and groups after importing nested repos', async () => {
    const importedRepo: Repo = {
      ...remoteRepo,
      id: 'local-imported',
      path: '/platform/api',
      projectGroupId: projectGroup.id,
      projectGroupOrder: 0
    }
    const result = {
      group: projectGroup,
      repos: [{ path: importedRepo.path, projectId: importedRepo.id, status: 'imported' as const }],
      importedCount: 1,
      alreadyKnownCount: 0,
      failedCount: 0
    }
    projectGroupsImportNested.mockResolvedValue(result)
    projectGroupsList.mockResolvedValue([projectGroup])
    folderWorkspacesList.mockResolvedValue([])
    reposList.mockResolvedValue([importedRepo])
    const store = createTestStore()

    await expect(
      store.getState().importNestedRepos({
        parentPath: '/platform',
        groupName: 'Platform',
        projectPaths: [importedRepo.path],
        mode: 'group'
      })
    ).resolves.toEqual(result)

    expect(projectGroupsImportNested).toHaveBeenCalledWith({
      parentPath: '/platform',
      groupName: 'Platform',
      projectPaths: [importedRepo.path],
      mode: 'group'
    })
    expect(projectGroupsList).toHaveBeenCalled()
    expect(folderWorkspacesList).toHaveBeenCalled()
    expect(reposList).toHaveBeenCalled()
    expect(store.getState().projectGroups).toEqual([{ ...projectGroup, executionHostId: 'local' }])
    // Why: the repos slice stamps fetched repos with their owning execution
    // host so multi-host routing never has to guess (multi-host design).
    expect(store.getState().repos).toEqual([{ ...importedRepo, executionHostId: 'local' }])
  })

  it('routes local nested scan progress by scanId and unsubscribes after completion', async () => {
    const unsubscribe = vi.fn()
    const progressCallback = vi.fn()
    const matchingScan = {
      selectedPath: '/platform',
      selectedPathKind: 'non_git_folder' as const,
      repos: [{ path: '/platform/api', displayName: 'api', depth: 1 }],
      truncated: false,
      timedOut: false,
      stopped: false,
      durationMs: 10,
      maxDepth: 3,
      maxRepos: 100,
      timeoutMs: null
    }
    projectGroupsOnNestedScanProgress.mockImplementation(
      (listener: (data: { scanId: string; scan: NestedRepoScanResult }) => void) => {
        listener({ scanId: 'other-scan', scan: { ...matchingScan, repos: [] } })
        listener({ scanId: 'scan-1', scan: matchingScan })
        return unsubscribe
      }
    )
    projectGroupsScanNested.mockResolvedValue(matchingScan)
    const store = createTestStore()

    await expect(
      store.getState().scanNestedRepos('/platform', undefined, {
        scanId: 'scan-1',
        onProgress: progressCallback
      })
    ).resolves.toEqual(matchingScan)

    expect(progressCallback).toHaveBeenCalledTimes(1)
    expect(progressCallback).toHaveBeenCalledWith(matchingScan)
    expect(projectGroupsScanNested).toHaveBeenCalledWith({
      path: '/platform',
      connectionId: undefined,
      scanId: 'scan-1'
    })
    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('unsubscribes local nested scan progress when the scan rejects', async () => {
    const unsubscribe = vi.fn()
    projectGroupsOnNestedScanProgress.mockReturnValue(unsubscribe)
    projectGroupsScanNested.mockRejectedValue(new Error('scan failed'))
    const store = createTestStore()

    await expect(
      store.getState().scanNestedRepos('/platform', undefined, {
        scanId: 'scan-1',
        onProgress: vi.fn()
      })
    ).resolves.toBeNull()

    expect(unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('cancels local nested scans through the preload API', async () => {
    projectGroupsCancelNestedScan.mockResolvedValue(true)
    const store = createTestStore()

    await expect(store.getState().cancelNestedRepoScan('scan-1')).resolves.toBe(true)

    expect(projectGroupsCancelNestedScan).toHaveBeenCalledWith({ scanId: 'scan-1' })
  })

  it('does not send cancelNestedRepoScan to a runtime environment transport', async () => {
    const store = createTestStore()
    store.setState({ settings: { activeRuntimeEnvironmentId: 'env-1' } as never })

    await expect(store.getState().cancelNestedRepoScan('scan-1')).resolves.toBe(false)

    expect(projectGroupsCancelNestedScan).not.toHaveBeenCalled()
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })

  it('maps the remote scanNested candidate array and keeps the RPC bounded', async () => {
    // Why a bare candidate array, not a pre-shaped NestedRepoScanResult:
    // project.proto's ScanNestedResponse is `{candidates: [...]}` — the Go
    // handler (channels_tenant_project.go) returns that array directly, no
    // scan-progress metadata at all (no truncated/timedOut/durationMs/
    // maxDepth this deployment can report — mapRemoteNestedScanCandidates
    // fills those with deterministic placeholders, not real telemetry).
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-scan',
      ok: true,
      result: [{ path: '/platform/api', suggestedName: 'api', isGitRepo: true }],
      _meta: { runtimeId: 'runtime-remote' }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      activeDevServerId: null
    })

    await expect(store.getState().scanNestedRepos('/platform')).resolves.toEqual({
      selectedPath: '/platform',
      selectedPathKind: 'non_git_folder',
      repos: [
        {
          path: '/platform/api',
          displayName: 'api',
          depth: 0,
          suggestedName: 'api',
          isGitRepo: true
        }
      ],
      truncated: false,
      timedOut: false,
      stopped: false,
      durationMs: 0,
      maxDepth: 0,
      maxRepos: 1,
      timeoutMs: null
    })

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'projectGroup.scanNested',
      params: { devServerId: null, rootPath: '/platform' },
      timeoutMs: 20_000
    })
  })

  it('imports nested repos through the remote projectGroup.importNested RPC', async () => {
    // Why {devServerId, parentGroupId, selected}, not {parentPath,
    // groupName, projectPaths, scanId, mode}: the Go handler decodes
    // ImportNestedRequest's real fields, and its response has no
    // group/mode concept at all — see mapRemoteImportNestedResult's doc
    // comment in repos.ts.
    runtimeEnvironmentCall.mockImplementation((request: RuntimeEnvironmentCallRequest) => {
      switch (request.method) {
        case 'projectGroup.importNested':
          return {
            id: 'rpc-import',
            ok: true,
            result: {
              createdGroups: [
                { id: 'group-2', name: 'api', parentGroupId: '', projectId: 'proj-1' }
              ],
              createdProjects: [{ id: 'proj-1', name: 'api' }]
            },
            _meta: { runtimeId: 'runtime-remote' }
          }
        case 'projectGroup.list':
          return { id: 'rpc-groups', ok: true, result: [], _meta: { runtimeId: 'runtime-remote' } }
        case 'folderWorkspace.list':
          return {
            id: 'rpc-fw',
            ok: true,
            result: { folderWorkspaces: [] },
            _meta: { runtimeId: 'runtime-remote' }
          }
        case 'repo.list':
          return {
            id: 'rpc-repos',
            ok: true,
            result: { repos: [] },
            _meta: { runtimeId: 'runtime-remote' }
          }
        case 'project.list':
          return { id: 'rpc-projs', ok: true, result: [], _meta: { runtimeId: 'runtime-remote' } }
        default:
          throw new Error(`Unexpected runtime method ${request.method}`)
      }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      activeDevServerId: 'ds-1'
    })

    await expect(
      store.getState().importNestedRepos({
        parentPath: '/platform',
        groupName: 'Platform',
        projectPaths: ['/platform/api'],
        mode: 'group',
        selectedCandidates: [
          {
            path: '/platform/api',
            displayName: 'api',
            depth: 0,
            suggestedName: 'api',
            isGitRepo: true
          }
        ]
      })
    ).resolves.toEqual({
      projects: [{ path: '/platform/api', projectId: 'proj-1', status: 'imported' }],
      importedCount: 1,
      alreadyKnownCount: 0,
      failedCount: 0
    })

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'projectGroup.importNested',
      params: {
        devServerId: 'ds-1',
        parentGroupId: '',
        selected: [{ path: '/platform/api', suggestedName: 'api', isGitRepo: true }]
      },
      timeoutMs: 60_000
    })
  })

  it('moves local repos to a group using the preload projectId contract', async () => {
    const movedRepo = { ...remoteRepo, projectGroupId: projectGroup.id, projectGroupOrder: 3 }
    projectGroupsMoveProject.mockResolvedValue(movedRepo)
    const store = createTestStore()
    store.setState({ repos: [remoteRepo], projectGroups: [projectGroup] })

    await expect(
      store.getState().moveProjectToGroup(remoteRepo.id, projectGroup.id, 3)
    ).resolves.toBe(true)

    expect(projectGroupsMoveProject).toHaveBeenCalledWith({
      projectId: remoteRepo.id,
      groupId: projectGroup.id,
      order: 3
    })
    // Why: the repos slice stamps updated repos with their owning execution
    // host so multi-host routing never has to guess (multi-host design).
    expect(store.getState().repos).toEqual([{ ...movedRepo, executionHostId: 'local' }])
  })

  it('propagates specific folder workspace create failures to callers', async () => {
    folderWorkspacesCreate.mockRejectedValue(new Error('folder_workspace_path_missing:/srv/app'))
    const store = createTestStore()

    await expect(
      store.getState().createFolderWorkspace({
        projectGroupId: projectGroup.id,
        name: 'Broken folder'
      })
    ).rejects.toThrow(
      'Folder not found. Orca cannot find /srv/app. Remove and re-import the folder.'
    )
  })
})
