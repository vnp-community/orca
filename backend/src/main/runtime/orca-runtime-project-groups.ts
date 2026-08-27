/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
project/project-group/folder-workspace/nested-repo-import command block,
already covered by orca-runtime.ts's own grandfathered max-lines disable
before this move. Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-project-groups.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-046): project/project-group/folder-
// workspace/nested-repo-import commands extracted from OrcaRuntimeService via
// the composition pattern. `addRepo`/`cloneRepo` (repo lifecycle) sit
// immediately after this block in the source but are a separate, larger
// domain — kept as host dependencies here rather than pulled in, to keep
// this a pure Move.
import { homedir } from 'node:os'
import { isAbsolute, resolve } from 'node:path'
import { readdir, stat } from 'node:fs/promises'
import { randomUUID } from 'node:crypto'
import type {
  DirEntry,
  FolderWorkspace,
  NestedRepoScanResult,
  Project,
  ProjectGroup,
  ProjectGroupImportMode,
  ProjectGroupImportResult,
  ProjectHostSetup,
  ProjectHostSetupCloneArgs,
  ProjectHostSetupCreateArgs,
  ProjectHostSetupCreateResult,
  ProjectHostSetupDeleteArgs,
  ProjectHostSetupDeleteResult,
  ProjectHostSetupExistingFolderArgs,
  ProjectHostSetupResult,
  ProjectHostSetupUpdateArgs,
  ProjectHostSetupUpdateResult,
  ProjectUpdateArgs,
  Repo
} from '../../shared/types'
import type {
  FolderWorkspacePathStatus,
  FolderWorkspacePathStatusRequest
} from '../../shared/folder-workspace-path-status'
import type { ExecutionHostId } from '../../shared/execution-host'
import { normalizeRuntimePathForComparison } from '../../shared/cross-platform-path'
import { normalizeSparseDirectories } from '../ipc/sparse-checkout-directories'
import { isGitRepo, getRepoName } from '../git/repo'
import { gitExecFileAsync } from '../git/runner'
import { DEFAULT_REPO_BADGE_COLOR } from '../../shared/constants'
import { getProjectHostSetupForRepo } from '../../shared/project-host-setup-projection'
import { prepareLocalWorktreeRootForRepo } from '../worktree-root-preparation'
import { invalidateAuthorizedRootsCache } from '../ipc/filesystem-auth'
import { getRemoteFilesystemProvider } from '../providers/ssh-filesystem-dispatch'
import {
  assertFolderWorkspacePathUsable,
  getFolderWorkspacePathStatus as getFolderWorkspacePathStatusImpl,
  getFolderWorkspacePathStatusForPath
} from '../project-groups/folder-workspace-path-status'
import { scanNestedRepos as scanNestedReposImpl } from '../project-groups/nested-repo-discovery'
import {
  createNestedProjectGroupResolver,
  resolveNestedRepoSelection
} from '../project-groups/nested-repo-import'
import { createNestedRepoImportTargetResolver } from '../project-groups/nested-repo-import-target'
import { enrichMissingRepoGitRemoteIdentities as enrichMissingRepoGitRemoteIdentitiesImpl } from '../repo-git-remote-identity-enrichment'
import type { RuntimeStore } from './orca-runtime'

// Why: only used by saveSparsePreset in this domain — moved bodily rather
// than kept shared, matching TASK-BIGFILE-044's sameStringSet/labelsForIds
// precedent for single-consumer local helpers.
function normalizeSparsePresetName(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) {
    throw new Error('Preset name is required.')
  }
  if (trimmed.length > 80) {
    throw new Error('Preset name is too long.')
  }
  return trimmed
}

function normalizeSparsePresetDirectoriesForSave(directories: string[]): string[] {
  let normalized: string[]
  try {
    normalized = normalizeSparseDirectories(directories)
  } catch (err) {
    if (
      err instanceof Error &&
      err.message === 'Sparse checkout directories must be repo-relative paths.'
    ) {
      throw new Error('Preset directories must be repo-relative paths.')
    }
    throw err
  }
  if (normalized.length === 0) {
    throw new Error('Preset must have at least one directory.')
  }
  return normalized
}

function sanitizeNestedRepoRuntimeImportError(context: string, error: unknown): string {
  console.warn(`[project-groups] ${context}`, error)
  return 'Repository could not be imported'
}

function resolveServerBrowsePath(pathValue: string): string {
  const trimmed = pathValue.trim() || '~'
  if (trimmed.includes('\0')) {
    throw new Error('Path cannot contain null bytes')
  }
  if (trimmed === '~') {
    return homedir()
  }
  if (/^~[\\/]/.test(trimmed)) {
    return resolve(homedir(), trimmed.slice(2))
  }
  if (isAbsolute(trimmed)) {
    return resolve(trimmed)
  }
  // Why: remote clients do not share the server process cwd; relative browse
  // inputs are anchored to the server user's home to match the `~` picker root.
  return resolve(homedir(), trimmed)
}

export type RuntimeProjectGroupsCommandHost = {
  getStore(): RuntimeStore | null
  resolveRepoSelector(selector: string): Promise<Repo>
  notifyReposChanged(): void
  invalidateResolvedWorktreeCache(): void
  addRepo(
    path: string,
    kind?: 'git' | 'folder',
    executionHostId?: ExecutionHostId | null
  ): Promise<Repo>
  cloneRepo(
    url: string,
    destination: string,
    executionHostId?: ExecutionHostId | null
  ): Promise<Repo>
}

export class RuntimeProjectGroupsCommands {
  constructor(private readonly host: RuntimeProjectGroupsCommandHost) {}

  listRepos(): Repo[] {
    return this.host.getStore()?.getRepos() ?? []
  }

  enrichMissingRepoGitRemoteIdentities(): void {
    const store = this.host.getStore()
    if (!store) {
      return
    }
    enrichMissingRepoGitRemoteIdentitiesImpl(store, {
      onChanged: () => {
        this.host.invalidateResolvedWorktreeCache()
        this.host.notifyReposChanged()
      }
    })
  }

  listProjects(): Project[] {
    return this.host.getStore()?.getProjects?.() ?? []
  }

  updateProject(projectId: string, updates: ProjectUpdateArgs['updates']): Project {
    const store = this.host.getStore()
    if (!store?.updateProject) {
      throw new Error('runtime_unavailable')
    }
    const project = store.updateProject(projectId, updates)
    if (!project) {
      throw new Error(`Project not found: ${projectId}`)
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    return project
  }

  listProjectHostSetups(): ProjectHostSetup[] {
    return this.host.getStore()?.getProjectHostSetups?.() ?? []
  }

  createProjectHostSetup(args: ProjectHostSetupCreateArgs): ProjectHostSetupCreateResult {
    const store = this.host.getStore()
    if (!store?.createProjectHostSetup) {
      throw new Error('runtime_unavailable')
    }
    const result = store.createProjectHostSetup(args)
    if (!result) {
      throw new Error(`Project not found: ${args.projectId}`)
    }
    return result
  }

  async setupProjectExistingFolder(
    args: ProjectHostSetupExistingFolderArgs
  ): Promise<ProjectHostSetupResult> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    let repo = await this.host.addRepo(
      args.path,
      args.kind === 'folder' ? 'folder' : 'git',
      args.hostId
    )
    let setup = getProjectHostSetupForRepo(this.listProjectHostSetups(), repo)
    if (setup.projectId !== args.projectId) {
      const existingProject = this.listProjects().find((project) => project.id === args.projectId)
      if (
        !existingProject?.providerIdentity ||
        existingProject.providerIdentity.provider !== 'github'
      ) {
        throw new Error('Imported folder does not match the selected project identity.')
      }
      const updated = store.updateRepo(repo.id, {
        upstream: {
          owner: existingProject.providerIdentity.owner,
          repo: existingProject.providerIdentity.repo
        }
      })
      if (!updated) {
        throw new Error(`Project setup repo disappeared before it could be linked: ${repo.id}`)
      }
      repo = updated
      setup = getProjectHostSetupForRepo(this.listProjectHostSetups(), repo)
    }
    const setupMethod = args.setupMethod ?? 'imported-existing-folder'
    const updated = store.updateRepo(repo.id, { projectHostSetupMethod: setupMethod })
    if (!updated) {
      throw new Error(
        `Project setup repo disappeared before setup metadata could be linked: ${repo.id}`
      )
    }
    repo = updated
    setup = getProjectHostSetupForRepo(this.listProjectHostSetups(), repo)
    const project = this.listProjects().find((entry) => entry.id === setup.projectId)
    if (!project) {
      throw new Error(`Project setup was created without a project record: ${setup.projectId}`)
    }
    return { project, setup, repo }
  }

  async setupProjectClone(args: ProjectHostSetupCloneArgs): Promise<ProjectHostSetupResult> {
    const repo = await this.host.cloneRepo(args.url, args.destination, args.hostId)
    return await this.setupProjectExistingFolder({
      projectId: args.projectId,
      hostId: args.hostId,
      path: repo.path,
      kind: 'git',
      displayName: args.displayName,
      setupMethod: 'cloned'
    })
  }

  updateProjectHostSetup(args: ProjectHostSetupUpdateArgs): ProjectHostSetupUpdateResult {
    const store = this.host.getStore()
    if (!store?.updateProjectHostSetup) {
      throw new Error('runtime_unavailable')
    }
    const result = store.updateProjectHostSetup(args)
    if (!result) {
      throw new Error(`Project host setup not found: ${args.setupId}`)
    }
    if ('worktreeBasePath' in args.updates && result.repo) {
      void prepareLocalWorktreeRootForRepo(store, result.repo)
      invalidateAuthorizedRootsCache()
    }
    return result
  }

  deleteProjectHostSetup(args: ProjectHostSetupDeleteArgs): ProjectHostSetupDeleteResult {
    const store = this.host.getStore()
    if (!store?.deleteProjectHostSetup) {
      throw new Error('runtime_unavailable')
    }
    const result = store.deleteProjectHostSetup(args)
    if (!result) {
      throw new Error(`Project host setup not found: ${args.setupId}`)
    }
    return result
  }

  listProjectGroups(): ProjectGroup[] {
    return this.host.getStore()?.getProjectGroups?.() ?? []
  }

  listFolderWorkspaces(): FolderWorkspace[] {
    return this.host.getStore()?.getFolderWorkspaces?.() ?? []
  }

  async createProjectGroup(input: {
    name: string
    parentPath?: string | null
    connectionId?: string | null
    parentGroupId?: string | null
    createdFrom?: ProjectGroup['createdFrom']
  }): Promise<ProjectGroup> {
    const store = this.host.getStore()
    if (!store?.createProjectGroup) {
      throw new Error('runtime_unavailable')
    }
    const group = store.createProjectGroup({
      name: input.name,
      parentPath: input.parentPath ?? null,
      connectionId: input.connectionId ?? null,
      parentGroupId: input.parentGroupId ?? null,
      createdFrom: input.createdFrom ?? 'manual'
    })
    this.host.notifyReposChanged()
    return group
  }

  async updateProjectGroup(
    groupId: string,
    updates: Partial<Pick<ProjectGroup, 'name' | 'isCollapsed' | 'tabOrder' | 'color'>>
  ): Promise<ProjectGroup | null> {
    const store = this.host.getStore()
    if (!store?.updateProjectGroup) {
      throw new Error('runtime_unavailable')
    }
    const updated = store.updateProjectGroup(groupId, updates)
    if (updated) {
      this.host.notifyReposChanged()
    }
    return updated
  }

  async deleteProjectGroup(groupId: string): Promise<{ deleted: boolean }> {
    const store = this.host.getStore()
    if (!store?.deleteProjectGroup) {
      throw new Error('runtime_unavailable')
    }
    const deleted = store.deleteProjectGroup(groupId)
    if (deleted) {
      this.host.notifyReposChanged()
    }
    return { deleted }
  }

  async moveProjectToGroup(
    repoSelector: string,
    groupId: string | null,
    order?: number
  ): Promise<Repo> {
    const store = this.host.getStore()
    if (!store?.moveProjectToGroup) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const moved = store.moveProjectToGroup(repo.id, groupId, order)
    if (!moved) {
      throw new Error('repo_not_found')
    }
    this.host.notifyReposChanged()
    return moved
  }

  async createFolderWorkspace(input: {
    projectGroupId: string
    name?: string
    folderPath?: string | null
    connectionId?: string | null
    linkedTask?: FolderWorkspace['linkedTask']
    createdWithAgent?: FolderWorkspace['createdWithAgent']
    pendingFirstAgentMessageRename?: boolean
  }): Promise<FolderWorkspace> {
    const store = this.host.getStore()
    if (!store?.createFolderWorkspace) {
      throw new Error('runtime_unavailable')
    }
    const projectGroups = store.getProjectGroups?.() ?? []
    const group = projectGroups.find((entry) => entry.id === input.projectGroupId)
    const folderPath =
      typeof input.folderPath === 'string' && input.folderPath.trim().length > 0
        ? input.folderPath
        : group?.parentPath
    if (!group || !folderPath) {
      throw new Error('folder_workspace_project_group_not_found')
    }
    const status = await getFolderWorkspacePathStatusForPath(
      {
        folderPath,
        projectGroupId: group.id,
        connectionId: input.connectionId ?? group.connectionId ?? null,
        projectGroups,
        repos: store.getRepos()
      },
      { getRemoteFilesystemProvider }
    )
    assertFolderWorkspacePathUsable(status)
    const workspace = store.createFolderWorkspace(input)
    this.host.notifyReposChanged()
    return workspace
  }

  async getFolderWorkspacePathStatus(
    request: FolderWorkspacePathStatusRequest
  ): Promise<FolderWorkspacePathStatus> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    return getFolderWorkspacePathStatusImpl(store, request, { getRemoteFilesystemProvider })
  }

  async updateFolderWorkspace(
    folderWorkspaceId: string,
    updates: Partial<
      Pick<
        FolderWorkspace,
        | 'name'
        | 'folderPath'
        | 'linkedTask'
        | 'comment'
        | 'isArchived'
        | 'isUnread'
        | 'isPinned'
        | 'sortOrder'
        | 'manualOrder'
        | 'workspaceStatus'
        | 'createdWithAgent'
        | 'pendingFirstAgentMessageRename'
        | 'firstAgentMessageRenameError'
        | 'lastActivityAt'
      >
    >
  ): Promise<FolderWorkspace | null> {
    const store = this.host.getStore()
    if (!store?.updateFolderWorkspace) {
      throw new Error('runtime_unavailable')
    }
    if (typeof updates.folderPath === 'string' && updates.folderPath.trim().length > 0) {
      const workspace = store
        .getFolderWorkspaces?.()
        .find((entry) => entry.id === folderWorkspaceId)
      if (!workspace) {
        return null
      }
      const projectGroups = store.getProjectGroups?.() ?? []
      const status = await getFolderWorkspacePathStatusForPath(
        {
          folderPath: updates.folderPath,
          projectGroupId: workspace.projectGroupId,
          connectionId:
            workspace.connectionId ??
            projectGroups.find((entry) => entry.id === workspace.projectGroupId)?.connectionId ??
            null,
          projectGroups,
          repos: store.getRepos()
        },
        { getRemoteFilesystemProvider }
      )
      assertFolderWorkspacePathUsable(status)
    }
    const updated = store.updateFolderWorkspace(folderWorkspaceId, updates)
    if (updated) {
      this.host.notifyReposChanged()
    }
    return updated
  }

  async deleteFolderWorkspace(folderWorkspaceId: string): Promise<{ deleted: boolean }> {
    const store = this.host.getStore()
    if (!store?.removeFolderWorkspace) {
      throw new Error('runtime_unavailable')
    }
    const deleted = store.removeFolderWorkspace(folderWorkspaceId)
    if (deleted) {
      this.host.notifyReposChanged()
    }
    return { deleted }
  }

  async scanNestedRepos(path: string): Promise<NestedRepoScanResult> {
    if (!isAbsolute(path)) {
      throw new Error('Project path must be an absolute path')
    }
    return scanNestedReposImpl({ path, options: { timeoutMs: 15_000 } })
  }

  async browseServerDir(pathValue: string): Promise<{ resolvedPath: string; entries: DirEntry[] }> {
    const dirPath = resolveServerBrowsePath(pathValue)
    const dirStat = await stat(dirPath)
    if (!dirStat.isDirectory()) {
      throw new Error(`${dirPath} is not a directory`)
    }
    const entries = await readdir(dirPath, { withFileTypes: true })
    const mapped = entries
      .filter((entry) => entry.name !== '.' && entry.name !== '..')
      .map((entry) => ({
        name: entry.name,
        isDirectory: entry.isDirectory(),
        isSymlink: entry.isSymbolicLink()
      }))
    mapped.sort((a, b) => {
      if (a.isDirectory !== b.isDirectory) {
        return a.isDirectory ? -1 : 1
      }
      return a.name.localeCompare(b.name)
    })
    return { resolvedPath: dirPath, entries: mapped }
  }

  async isGitAvailable(): Promise<boolean> {
    try {
      await gitExecFileAsync(['--version'], { cwd: process.cwd(), timeout: 3000 })
      return true
    } catch {
      return false
    }
  }

  async importNestedRepos(args: {
    parentPath: string
    groupName: string
    projectPaths: string[]
    mode: ProjectGroupImportMode
  }): Promise<ProjectGroupImportResult> {
    const store = this.host.getStore()
    if (!store?.createProjectGroup || !store?.moveProjectToGroup) {
      throw new Error('runtime_unavailable')
    }
    if (!isAbsolute(args.parentPath)) {
      throw new Error('Project path must be an absolute path')
    }
    const scan = await scanNestedReposImpl({
      path: args.parentPath,
      options: { timeoutMs: 15_000 }
    })
    const selection = resolveNestedRepoSelection({ scan, projectPaths: args.projectPaths })
    const groupResolver = createNestedProjectGroupResolver({
      parentPath: args.parentPath,
      groupName: args.groupName,
      mode: args.mode,
      connectionId: null,
      repoPaths: selection.selectedPaths,
      createGroup: (input) => store.createProjectGroup!(input)
    })
    const results: ProjectGroupImportResult['projects'] = selection.rejectedPaths.map(
      (repoPath) => ({
        path: repoPath,
        status: 'failed',
        error: 'Repository was not found in the nested repo scan result'
      })
    )
    const importedProjectIdsByRepoPath = new Map<string, string>()
    const importTargetResolver = createNestedRepoImportTargetResolver()
    for (const [projectGroupOrder, repoPath] of selection.selectedPaths.entries()) {
      try {
        if (!isGitRepo(repoPath)) {
          results.push({ path: repoPath, status: 'failed', error: 'Not a valid git repository' })
          continue
        }
        const importRepoPath = await importTargetResolver.resolveLocal(repoPath)
        const normalizedImportRepoPath = normalizeRuntimePathForComparison(importRepoPath)
        const alreadyImportedProjectId = importedProjectIdsByRepoPath.get(normalizedImportRepoPath)
        if (alreadyImportedProjectId) {
          results.push({
            path: repoPath,
            projectId: alreadyImportedProjectId,
            status: 'already-known'
          })
          continue
        }
        const existing = store
          .getRepos()
          .find((repo) => normalizeRuntimePathForComparison(repo.path) === normalizedImportRepoPath)
        const group = groupResolver.getGroupForRepo(repoPath)
        if (existing) {
          if (group) {
            store.moveProjectToGroup(existing.id, group.id, projectGroupOrder)
          }
          importedProjectIdsByRepoPath.set(normalizedImportRepoPath, existing.id)
          results.push({ path: repoPath, projectId: existing.id, status: 'already-known' })
          continue
        }
        const repo: Repo = {
          id: randomUUID(),
          path: importRepoPath,
          displayName: getRepoName(importRepoPath),
          badgeColor: DEFAULT_REPO_BADGE_COLOR,
          addedAt: Date.now(),
          kind: 'git',
          externalWorktreeVisibility: 'hide',
          externalWorktreeVisibilityLegacy: false,
          ...(group
            ? {
                projectGroupId: group.id,
                projectGroupOrder
              }
            : {})
        }
        store.addRepo(repo)
        importedProjectIdsByRepoPath.set(normalizedImportRepoPath, repo.id)
        results.push({ path: repoPath, projectId: repo.id, status: 'imported' })
      } catch (error) {
        results.push({
          path: repoPath,
          status: 'failed',
          error: sanitizeNestedRepoRuntimeImportError(
            'Failed to import nested repository in runtime',
            error
          )
        })
      }
    }
    const importedCount = results.filter((entry) => entry.status === 'imported').length
    const alreadyKnownCount = results.filter((entry) => entry.status === 'already-known').length
    const failedCount = results.filter((entry) => entry.status === 'failed').length
    if (importedCount + alreadyKnownCount === 0) {
      for (const group of groupResolver.getCreatedGroups().toReversed()) {
        store.deleteProjectGroup?.(group.id)
      }
    }
    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyReposChanged()
    const rootGroup = groupResolver.getRootGroup()
    return {
      ...(rootGroup && importedCount + alreadyKnownCount > 0 ? { group: rootGroup } : {}),
      projects: results,
      importedCount,
      alreadyKnownCount,
      failedCount
    }
  }

  async listSparsePresets(repoSelector: string) {
    const store = this.host.getStore()
    if (!store?.getSparsePresets) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    return store.getSparsePresets(repo.id)
  }

  async saveSparsePreset(
    repoSelector: string,
    args: { id?: string; name: string; directories: string[] }
  ) {
    const store = this.host.getStore()
    if (!store?.getSparsePresets || !store.saveSparsePreset) {
      throw new Error('runtime_unavailable')
    }
    const repo = await this.host.resolveRepoSelector(repoSelector)
    const name = normalizeSparsePresetName(args.name)
    const directories = normalizeSparsePresetDirectoriesForSave(args.directories)
    const now = Date.now()
    const existing = args.id
      ? store.getSparsePresets(repo.id).find((preset) => preset.id === args.id)
      : undefined
    return store.saveSparsePreset({
      id: existing?.id ?? randomUUID(),
      repoId: repo.id,
      name,
      directories,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now
    })
  }
}
