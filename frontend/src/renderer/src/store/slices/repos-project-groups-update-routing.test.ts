import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestStore } from './store-test-helpers'
import type { ProjectGroup, FolderWorkspace } from '../../../../shared/types'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'

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

// Why these tests: project.proto's ProjectGroup/UpdateProjectGroupRequest
// have no tabOrder/isCollapsed/color fields, and its usecase rejects an
// empty name outright (PROJECT_GROUP_INVALID) — updateProjectGroup's
// remote path used to send {groupId, updates} unconditionally (the Go
// handler decodes {groupId, name}), so a real rename never actually
// reached the backend and a tabOrder-only reorder always failed
// server-side. Fixed alongside these tests.
describe('updateProjectGroup remote routing', () => {
  it('renames via projectGroup.update, preserving local-only fields the backend response omits', async () => {
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-update-group',
      ok: true,
      result: {
        id: projectGroup.id,
        name: 'Renamed',
        parentGroupId: null,
        projectId: ''
      },
      _meta: { runtimeId: 'runtime-remote' }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      projectGroups: [{ ...projectGroup, tabOrder: 3, isCollapsed: true, color: 'blue' }]
    })

    await expect(
      store.getState().updateProjectGroup(projectGroup.id, { name: 'Renamed' })
    ).resolves.toBe(true)

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'projectGroup.update',
      params: { groupId: projectGroup.id, name: 'Renamed' },
      timeoutMs: 15_000
    })
    const updated = store.getState().projectGroups[0]
    expect(updated.name).toBe('Renamed')
    // Local-only fields the backend response has no concept of at all —
    // must survive the merge, not get wiped by a wholesale replace.
    expect(updated.tabOrder).toBe(3)
    expect(updated.isCollapsed).toBe(true)
    expect(updated.color).toBe('blue')
  })

  it('applies a tabOrder-only change locally, without calling projectGroup.update at all', async () => {
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      projectGroups: [projectGroup]
    })

    await expect(
      store.getState().updateProjectGroup(projectGroup.id, { tabOrder: 5 })
    ).resolves.toBe(true)

    expect(store.getState().projectGroups[0].tabOrder).toBe(5)
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })
})

// Why: folder-workspace.update's Go handler returns resp.GetFolderWorkspace()
// bare (no {folderWorkspace: ...} wrapper) and decodes {id, name} — the
// previous {folderWorkspaceId, updates} shape never actually renamed
// anything remotely; folderWorkspace.delete decodes {id}, not
// {folderWorkspaceId}, and returns {"ok": ...}, not {"deleted": ...}.
describe('updateFolderWorkspace / deleteFolderWorkspace remote routing', () => {
  const remoteFolderWorkspace: FolderWorkspace = {
    id: 'folder-workspace-remote',
    projectGroupId: projectGroup.id,
    name: 'Old name',
    folderPath: '/workspace/platform',
    linkedTask: null,
    comment: 'keep me',
    isArchived: false,
    isUnread: false,
    isPinned: false,
    sortOrder: 1,
    lastActivityAt: 0,
    createdAt: 1,
    updatedAt: 1
  }

  it('renames via folderWorkspace.update, merging the bare response over local-only fields', async () => {
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-update-folder-workspace',
      ok: true,
      result: {
        id: remoteFolderWorkspace.id,
        devServerId: 'dev-1',
        path: remoteFolderWorkspace.folderPath,
        name: 'New name',
        addedBy: 'user-1',
        createdAt: 1
      },
      _meta: { runtimeId: 'runtime-remote' }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      folderWorkspaces: [remoteFolderWorkspace]
    })

    await expect(
      store.getState().updateFolderWorkspace(remoteFolderWorkspace.id, { name: 'New name' })
    ).resolves.toBe(true)

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'folderWorkspace.update',
      params: { id: remoteFolderWorkspace.id, name: 'New name' },
      timeoutMs: 15_000
    })
    const updated = store.getState().folderWorkspaces[0]
    expect(updated.name).toBe('New name')
    expect(updated.comment).toBe('keep me')
  })

  it('applies a comment-only change locally, without calling folderWorkspace.update at all', async () => {
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      folderWorkspaces: [remoteFolderWorkspace]
    })

    await expect(
      store
        .getState()
        .updateFolderWorkspace(remoteFolderWorkspace.id, { comment: 'updated remotely' })
    ).resolves.toBe(true)

    expect(store.getState().folderWorkspaces[0].comment).toBe('updated remotely')
    expect(runtimeEnvironmentCall).not.toHaveBeenCalled()
  })

  it('deletes via folderWorkspace.delete using {id} and reads the {ok} response key', async () => {
    runtimeEnvironmentCall.mockResolvedValue({
      id: 'rpc-delete-folder-workspace',
      ok: true,
      result: { ok: true },
      _meta: { runtimeId: 'runtime-remote' }
    })
    const store = createTestStore()
    store.setState({
      settings: { activeRuntimeEnvironmentId: 'env-1' } as never,
      folderWorkspaces: [remoteFolderWorkspace]
    })

    await expect(store.getState().deleteFolderWorkspace(remoteFolderWorkspace.id)).resolves.toBe(
      true
    )

    expect(runtimeEnvironmentCall).toHaveBeenCalledWith({
      selector: 'env-1',
      method: 'folderWorkspace.delete',
      params: { id: remoteFolderWorkspace.id },
      timeoutMs: 15_000
    })
    expect(store.getState().folderWorkspaces).toEqual([])
  })
})
