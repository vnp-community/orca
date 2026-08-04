import type { AppState } from '@/store/types'
import { getIndexedRepoMap, getIndexedWorktreeMap } from '@/store/worktree-repo-index'
import { FLOATING_TERMINAL_WORKTREE_ID } from '../../../shared/constants'
import { getRepoIdFromWorktreeId } from '../../../shared/worktree-id'
import { parseWorkspaceKey } from '../../../shared/workspace-scope'
import { getRepoProviderConnectionKey } from '../../../shared/execution-host'
import {
  isPathInsideOrEqual,
  normalizeRuntimePathForComparison
} from '../../../shared/cross-platform-path'
import {
  getFolderWorkspaceCandidateRepos,
  getFolderWorkspaceConnectionId
} from './folder-workspace-connection'

type ConnectionOwnerState = Pick<
  AppState,
  'folderWorkspaces' | 'projectGroups' | 'repos' | 'worktreesByRepo'
>

export function createConnectionIdForFileSelector(
  worktreeId: string | null,
  filePath: string,
  { skip = false }: { skip?: boolean } = {}
): (state: ConnectionOwnerState) => string | null | undefined {
  let previousSlices: ConnectionOwnerState | null = null
  let previousResult: string | null | undefined
  return (state) => {
    if (skip) {
      return undefined
    }
    if (
      previousSlices?.folderWorkspaces === state.folderWorkspaces &&
      previousSlices.projectGroups === state.projectGroups &&
      previousSlices.repos === state.repos &&
      previousSlices.worktreesByRepo === state.worktreesByRepo
    ) {
      return previousResult
    }
    previousSlices = {
      folderWorkspaces: state.folderWorkspaces,
      projectGroups: state.projectGroups,
      repos: state.repos,
      worktreesByRepo: state.worktreesByRepo
    }
    previousResult = getConnectionIdForFileFromState(state, worktreeId, filePath)
    return previousResult
  }
}

export function getConnectionIdFromState(
  state: ConnectionOwnerState,
  worktreeId: string | null
): string | null | undefined {
  if (!worktreeId || worktreeId === FLOATING_TERMINAL_WORKTREE_ID) {
    return null
  }
  const parsedWorkspaceKey = parseWorkspaceKey(worktreeId)
  if (parsedWorkspaceKey?.type === 'folder') {
    return getFolderWorkspaceConnectionId(state, parsedWorkspaceKey.folderWorkspaceId)
  }
  // Why: owner resolution runs from retained Zustand selectors, so unrelated
  // store writes must not flatten every worktree or scan every repository.
  const worktree = getIndexedWorktreeMap(state.worktreesByRepo).get(worktreeId)
  const repoId = worktree?.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  const repo = getIndexedRepoMap(state.repos).get(repoId)
  return repo ? getRepoProviderConnectionKey(repo) : undefined
}

/**
 * SSH-target-only variant of getConnectionIdFromState — returns the repo's
 * `connectionId` (a real, configured SSH Target) and never `devServerId`.
 * Why a separate function: getConnectionIdFromState's generic provider key is
 * correct for IPC/PTY routing (SSH and Dev Server both resolve through the
 * same provider registries), but SSH-specific UI (the reconnect overlay,
 * "SSH host removed" detection) must not mistake a devServerId for a
 * ghost/removed SSH target — a Dev Server was never in the SSH targets list
 * to begin with.
 */
export function getSshConnectionIdFromState(
  state: ConnectionOwnerState,
  worktreeId: string | null
): string | null | undefined {
  if (!worktreeId || worktreeId === FLOATING_TERMINAL_WORKTREE_ID) {
    return null
  }
  const parsedWorkspaceKey = parseWorkspaceKey(worktreeId)
  if (parsedWorkspaceKey?.type === 'folder') {
    return getFolderWorkspaceConnectionId(state, parsedWorkspaceKey.folderWorkspaceId)
  }
  const worktree = getIndexedWorktreeMap(state.worktreesByRepo).get(worktreeId)
  const repoId = worktree?.repoId ?? getRepoIdFromWorktreeId(worktreeId)
  const repo = getIndexedRepoMap(state.repos).get(repoId)
  return repo ? (repo.connectionId ?? null) : undefined
}

export function getConnectionIdForFileFromState(
  state: ConnectionOwnerState,
  worktreeId: string | null,
  filePath: string
): string | null | undefined {
  const connectionId = getConnectionIdFromState(state, worktreeId)
  if (connectionId !== undefined || !worktreeId) {
    return connectionId
  }
  const parsedWorkspaceKey = parseWorkspaceKey(worktreeId)
  if (parsedWorkspaceKey?.type !== 'folder') {
    return undefined
  }
  const candidateRepos = getFolderWorkspaceCandidateRepos(
    state,
    parsedWorkspaceKey.folderWorkspaceId
  )
  const matchingRepos = candidateRepos
    .filter((repo) => isPathInsideOrEqual(repo.path, filePath))
    .map((repo) => ({ repo, normalizedPath: normalizeRuntimePathForComparison(repo.path) }))
    .sort((left, right) => right.normalizedPath.length - left.normalizedPath.length)
  const longestPathLength = matchingRepos[0]?.normalizedPath.length
  if (!longestPathLength) {
    return undefined
  }
  const connectionIds = new Set(
    matchingRepos
      .filter((candidate) => candidate.normalizedPath.length === longestPathLength)
      .map(({ repo }) => getRepoProviderConnectionKey(repo))
  )
  return connectionIds.size === 1 ? ([...connectionIds][0] ?? null) : undefined
}
