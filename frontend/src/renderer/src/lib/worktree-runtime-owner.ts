import {
  getRepoExecutionHostId,
  parseExecutionHostId,
  toSshExecutionHostId
} from '../../../shared/execution-host'
import type { ExecutionHostId, ParsedExecutionHost } from '../../../shared/execution-host'
import type {
  FolderWorkspace,
  GlobalSettings,
  ProjectGroup,
  Repo,
  Worktree
} from '../../../shared/types'
import { folderWorkspaceKey, parseWorkspaceKey } from '../../../shared/workspace-scope'
import { FLOATING_TERMINAL_WORKTREE_ID } from '../../../shared/constants'
import { getRepoIdFromWorktreeId } from '@/store/slices/worktree-helpers'
import {
  findIndexedFolderWorkspaceOwner,
  findIndexedProjectGroupOwner,
  findIndexedRepoOwner as findRepoRecord,
  findIndexedWorktreeOwner as findWorktreeRecord
} from './worktree-runtime-owner-index'

type RuntimeExecutionHost = Extract<ParsedExecutionHost, { kind: 'runtime' }>

export type WorktreeRuntimeOwnerState = {
  repos?: readonly Pick<Repo, 'id' | 'connectionId' | 'executionHostId' | 'devServerId'>[]
  settings?: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null
  worktreesByRepo?: Record<string, readonly Pick<Worktree, 'id' | 'repoId' | 'hostId'>[]>
  folderWorkspaces?: readonly Pick<FolderWorkspace, 'id' | 'projectGroupId' | 'connectionId'>[]
  projectGroups?: readonly Pick<ProjectGroup, 'id' | 'connectionId' | 'executionHostId'>[]
  restoredRuntimeHostIdByWorkspaceSessionKey?: Record<string, ExecutionHostId>
}

function findFolderProjectGroup(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): Pick<ProjectGroup, 'id' | 'connectionId' | 'executionHostId'> | null {
  const folderWorkspace = findFolderWorkspace(state, folderWorkspaceId)
  if (!folderWorkspace) {
    return null
  }
  return findIndexedProjectGroupOwner(state.projectGroups, folderWorkspace.projectGroupId)
}

function getGlobalActiveRuntimeEnvironmentId(state: WorktreeRuntimeOwnerState): string | null {
  const envId = state.settings?.activeRuntimeEnvironmentId?.trim()
  if (envId) {
    return envId
  }
  if (typeof window !== 'undefined' && (window as any)?.__orca_platform === 'web') {
    return 'session-auth'
  }
  return null
}

function findFolderWorkspace(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): Pick<FolderWorkspace, 'id' | 'projectGroupId' | 'connectionId'> | null {
  return findIndexedFolderWorkspaceOwner(state.folderWorkspaces, folderWorkspaceId)
}

function getRuntimeEnvironmentIdForFolderWorkspace(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): string | null {
  const folderWorkspace = findFolderWorkspace(state, folderWorkspaceId)
  const projectGroup = findFolderProjectGroup(state, folderWorkspaceId)
  const parsed = parseExecutionHostId(projectGroup?.executionHostId)
  if (parsed?.kind === 'runtime') {
    return parsed.environmentId
  }
  if (
    parsed?.kind === 'local' ||
    parsed?.kind === 'ssh' ||
    folderWorkspace?.connectionId?.trim() ||
    projectGroup?.connectionId?.trim()
  ) {
    return null
  }
  const restoredRuntimeHost = getRestoredRuntimeHostForFolderWorkspace(state, folderWorkspaceId)
  if (restoredRuntimeHost) {
    return restoredRuntimeHost.environmentId
  }
  return getGlobalActiveRuntimeEnvironmentId(state)
}

function getRestoredRuntimeHostForFolderWorkspace(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): RuntimeExecutionHost | null {
  // Why: runtime folder catalogs load after session hydration; the saved
  // per-host session partition is the only owner evidence during that gap.
  const workspaceKey = folderWorkspaceKey(folderWorkspaceId)
  const parsed = parseExecutionHostId(
    state.restoredRuntimeHostIdByWorkspaceSessionKey?.[workspaceKey]
  )
  return parsed?.kind === 'runtime' ? parsed : null
}

function getExplicitRuntimeEnvironmentIdFromHost(
  executionHostId: string | null | undefined
): string | null {
  const parsed = parseExecutionHostId(executionHostId)
  return parsed?.kind === 'runtime' ? parsed.environmentId : null
}

function getRuntimeEnvironmentIdFromWorktreeHost(
  hostId: string | null | undefined
): string | null | undefined {
  if (!hostId?.trim()) {
    return undefined
  }
  return getExplicitRuntimeEnvironmentIdFromHost(hostId)
}

function getExecutionHostIdFromWorktreeHost(
  hostId: string | null | undefined
): ExecutionHostId | null {
  return parseExecutionHostId(hostId)?.id ?? null
}

function getExplicitRuntimeEnvironmentIdForFolderWorkspace(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): string | null {
  const folderWorkspace = findFolderWorkspace(state, folderWorkspaceId)
  const projectGroup = findFolderProjectGroup(state, folderWorkspaceId)
  const parsed = parseExecutionHostId(projectGroup?.executionHostId)
  if (parsed) {
    return parsed.kind === 'runtime' ? parsed.environmentId : null
  }
  if (folderWorkspace?.connectionId?.trim() || projectGroup?.connectionId?.trim()) {
    return null
  }
  return getRestoredRuntimeHostForFolderWorkspace(state, folderWorkspaceId)?.environmentId ?? null
}

function getExecutionHostIdForFolderWorkspace(
  state: WorktreeRuntimeOwnerState,
  folderWorkspaceId: string
): ExecutionHostId {
  const folderWorkspace = findFolderWorkspace(state, folderWorkspaceId)
  const projectGroup = findFolderProjectGroup(state, folderWorkspaceId)
  const parsed = parseExecutionHostId(projectGroup?.executionHostId)
  if (parsed) {
    return parsed.id
  }
  const connectionId = folderWorkspace?.connectionId?.trim() || projectGroup?.connectionId?.trim()
  if (connectionId) {
    return toSshExecutionHostId(connectionId)
  }
  const restoredRuntimeHost = getRestoredRuntimeHostForFolderWorkspace(state, folderWorkspaceId)
  if (restoredRuntimeHost) {
    return restoredRuntimeHost.id
  }
  const environmentId = getGlobalActiveRuntimeEnvironmentId(state)
  return environmentId ? `runtime:${encodeURIComponent(environmentId)}` : 'local'
}

export function getRuntimeEnvironmentIdForWorktree(
  state: WorktreeRuntimeOwnerState,
  worktreeId: string | null | undefined
): string | null {
  if (!worktreeId) {
    return null
  }
  if (worktreeId === FLOATING_TERMINAL_WORKTREE_ID) {
    return null
  }
  const workspaceScope = parseWorkspaceKey(worktreeId)
  if (workspaceScope?.type === 'folder') {
    return getRuntimeEnvironmentIdForFolderWorkspace(state, workspaceScope.folderWorkspaceId)
  }
  const worktree = findWorktreeRecord(state.worktreesByRepo, worktreeId)
  const worktreeRuntimeEnvironmentId = getRuntimeEnvironmentIdFromWorktreeHost(worktree?.hostId)
  if (worktreeRuntimeEnvironmentId !== undefined) {
    // Why: the same repo can exist on local and remote hosts; a concrete
    // worktree host must override the repo-level default owner.
    return worktreeRuntimeEnvironmentId
  }
  const repoId = worktree?.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  const repo = findRepoRecord(state.repos, repoId)
  const hasExplicitOwner = Boolean(
    repo?.executionHostId?.trim() || repo?.connectionId?.trim() || repo?.devServerId?.trim()
  )
  if (repo && hasExplicitOwner) {
    const parsed = parseExecutionHostId(getRepoExecutionHostId(repo))
    return parsed?.kind === 'runtime' ? parsed.environmentId : null
  }
  return getGlobalActiveRuntimeEnvironmentId(state)
}

/**
 * Whether enough store data has loaded to confidently tell "this worktree has
 * no runtime-environment owner" apart from "we simply haven't fetched its
 * repo record yet".
 *
 * FIX BUG-FE-PTY-001: right after a page refresh, ensureWorktreeHasInitialTerminal
 * (worktree-activation.ts) used to fall straight through to "no owner, spawn a
 * local terminal" the instant getRuntimeEnvironmentIdForWorktree() returned
 * null — which it also does when the repo record for this worktree simply
 * hasn't loaded into the store yet (repos/worktreesByRepo mid-fetch). That
 * local terminal.create then raced the real Dev-Server session mirror landing
 * a few hundred ms later: shouldReplaceTerminalTab() (web-session-tabs-sync.ts)
 * correctly recognizes the local placeholder as redundant once mirrored PTYs
 * arrive and replaces it, but the local terminal.create had already resolved
 * server-side by then, so its PTY gets destroyed the instant it's created —
 * surfacing as SSH_SESSION_EXPIRED on the very next open.
 *
 * Mirrors getRuntimeEnvironmentIdForWorktree()'s own lookup order so a caller
 * can gate "is it safe to treat null as 'genuinely no owner'" without this
 * function's result ever disagreeing with what that lookup would find.
 */
export function isRepoOwnerDataLoadedForWorktree(
  state: WorktreeRuntimeOwnerState,
  worktreeId: string | null | undefined
): boolean {
  if (!worktreeId || worktreeId === FLOATING_TERMINAL_WORKTREE_ID) {
    return true
  }
  const workspaceScope = parseWorkspaceKey(worktreeId)
  if (workspaceScope?.type === 'folder') {
    // Why: no race reported for folder workspaces — keep this fix scoped to
    // the repo/worktree lookup path that BUG-FE-PTY-001 actually hit.
    return true
  }
  const worktree = findWorktreeRecord(state.worktreesByRepo, worktreeId)
  if (!worktree) {
    return false
  }
  if (getRuntimeEnvironmentIdFromWorktreeHost(worktree.hostId) !== undefined) {
    // Why: a concrete worktree-level host already resolves ownership on its
    // own (see getRuntimeEnvironmentIdForWorktree above) — the repo record
    // is never consulted in that case, so its load state doesn't matter here.
    return true
  }
  const repoId = worktree.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  return findRepoRecord(state.repos, repoId) !== null
}

export function getExplicitRuntimeEnvironmentIdForWorktree(
  state: WorktreeRuntimeOwnerState,
  worktreeId: string | null | undefined
): string | null {
  if (!worktreeId) {
    return null
  }
  const workspaceScope = parseWorkspaceKey(worktreeId)
  if (workspaceScope?.type === 'folder') {
    return getExplicitRuntimeEnvironmentIdForFolderWorkspace(
      state,
      workspaceScope.folderWorkspaceId
    )
  }
  const worktree = findWorktreeRecord(state.worktreesByRepo, worktreeId)
  if (worktree?.hostId) {
    return getExplicitRuntimeEnvironmentIdFromHost(worktree.hostId)
  }
  const repoId = worktree?.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  const repo = findRepoRecord(state.repos, repoId)
  if (!repo) {
    return null
  }
  // Why: session mirroring is expensive; a merely focused runtime must not make
  // legacy/local worktrees look remote-owned.
  return getExplicitRuntimeEnvironmentIdFromHost(getRepoExecutionHostId(repo))
}

export function getRuntimeSessionMirrorEnvironmentIds(state: WorktreeRuntimeOwnerState): string[] {
  const ids = new Set<string>()
  const activeRuntimeEnvironmentId = getGlobalActiveRuntimeEnvironmentId(state)
  if (activeRuntimeEnvironmentId) {
    ids.add(activeRuntimeEnvironmentId)
  }
  for (const repo of state.repos ?? []) {
    const environmentId = getExplicitRuntimeEnvironmentIdFromHost(getRepoExecutionHostId(repo))
    if (environmentId) {
      ids.add(environmentId)
    }
  }
  for (const worktrees of Object.values(state.worktreesByRepo ?? {})) {
    for (const worktree of worktrees) {
      const environmentId = getRuntimeEnvironmentIdFromWorktreeHost(worktree.hostId)
      if (environmentId) {
        ids.add(environmentId)
      }
    }
  }
  for (const group of state.projectGroups ?? []) {
    const environmentId = getExplicitRuntimeEnvironmentIdFromHost(group.executionHostId)
    if (environmentId) {
      ids.add(environmentId)
    }
  }
  for (const hostId of Object.values(state.restoredRuntimeHostIdByWorkspaceSessionKey ?? {})) {
    const parsed = parseExecutionHostId(hostId)
    if (parsed?.kind === 'runtime') {
      ids.add(parsed.environmentId)
    }
  }
  return [...ids].sort()
}

export function getExecutionHostIdForWorktree(
  state: WorktreeRuntimeOwnerState,
  worktreeId: string | null | undefined
): ExecutionHostId {
  if (!worktreeId) {
    return 'local'
  }
  if (worktreeId === FLOATING_TERMINAL_WORKTREE_ID) {
    return 'local'
  }
  const workspaceScope = parseWorkspaceKey(worktreeId)
  if (workspaceScope?.type === 'folder') {
    return getExecutionHostIdForFolderWorkspace(state, workspaceScope.folderWorkspaceId)
  }
  const worktree = findWorktreeRecord(state.worktreesByRepo, worktreeId)
  const worktreeHostId = getExecutionHostIdFromWorktreeHost(worktree?.hostId)
  if (worktreeHostId) {
    // Why: per-worktree host ownership is more specific than the repo host
    // default, especially when local and runtime checkouts share a project.
    return worktreeHostId
  }
  const repoId = worktree?.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  const repo = findRepoRecord(state.repos, repoId)
  const hasExplicitOwner = Boolean(
    repo?.executionHostId?.trim() || repo?.connectionId?.trim() || repo?.devServerId?.trim()
  )
  if (repo && hasExplicitOwner) {
    return getRepoExecutionHostId(repo)
  }
  const environmentId = getGlobalActiveRuntimeEnvironmentId(state)
  return environmentId ? `runtime:${encodeURIComponent(environmentId)}` : 'local'
}

export function getSettingsForWorktreeRuntimeOwner(
  state: WorktreeRuntimeOwnerState,
  worktreeId: string | null | undefined
): Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> {
  return {
    ...state.settings,
    activeRuntimeEnvironmentId: getRuntimeEnvironmentIdForWorktree(state, worktreeId)
  }
}
