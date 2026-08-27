import { useCallback, useEffect, useRef, useState } from 'react'
import { getConnectionId } from '@/lib/connection-context'
import { getRuntimeGitHistory } from '@/runtime/runtime-git-client'
import type { GitHistoryPanelState } from './GitHistoryPanel'
import type { GlobalSettings, Worktree } from '../../../../shared/types'

const EMPTY_GIT_HISTORY_STATE: GitHistoryPanelState = { status: 'idle' }

export type UseSourceControlGitHistoryRefreshInput = {
  activeWorktreeId: string | null
  worktreePath: string | null
  isFolder: boolean
  isBranchVisible: boolean
  compareBaseRef: string | null
  activeRepoSettings: GlobalSettings | null
  isGitHistoryExpanded: boolean
  isGitHistoryVisible: boolean
  // Why: not in TASK-BIGFILE-226's original input list — found via grep that
  // the shared worktree-removal pruning effect in SourceControlInner also
  // wrote this cluster's per-worktree git-history state. Pruning now runs
  // here via its own worktreeMap-triggered effect instead.
  worktreeMap: Map<string, Worktree>
}

/**
 * Git-history refresh scheduling. Split out of
 * useSourceControlBranchCompareRefresh (TASK-BIGFILE-226), which calls this
 * internally — purely to stay under the 300-line file cap, no behavior
 * change from the original single cluster.
 */
export function useSourceControlGitHistoryRefresh(input: UseSourceControlGitHistoryRefreshInput) {
  const {
    activeWorktreeId,
    worktreePath,
    isFolder,
    isBranchVisible,
    compareBaseRef,
    activeRepoSettings,
    isGitHistoryExpanded,
    isGitHistoryVisible,
    worktreeMap
  } = input

  const [gitHistoryByWorktree, setGitHistoryByWorktree] = useState<
    Record<string, GitHistoryPanelState>
  >({})
  const gitHistoryRequestSeqRef = useRef(0)
  const gitHistoryRequestByWorktreeRef = useRef<Record<string, number>>({})
  const gitHistoryState = activeWorktreeId
    ? (gitHistoryByWorktree[activeWorktreeId] ?? EMPTY_GIT_HISTORY_STATE)
    : EMPTY_GIT_HISTORY_STATE

  // Why: orphaned git-history entries accumulate when worktrees are removed
  // from the store (long sessions with many create/destroy cycles). Mirrors
  // the sibling pruning effect that stayed in SourceControlInner for its
  // other per-worktree state slices (same trigger: worktreeMap).
  useEffect(() => {
    setGitHistoryByWorktree((prev) => {
      let changed = false
      const next: Record<string, GitHistoryPanelState> = {}
      for (const key of Object.keys(prev)) {
        if (worktreeMap.has(key)) {
          next[key] = prev[key]
        } else {
          changed = true
        }
      }
      return changed ? next : prev
    })
    for (const key of Object.keys(gitHistoryRequestByWorktreeRef.current)) {
      if (!worktreeMap.has(key)) {
        delete gitHistoryRequestByWorktreeRef.current[key]
      }
    }
  }, [worktreeMap])

  const refreshGitHistory = useCallback(async (): Promise<void> => {
    if (
      !activeWorktreeId ||
      !worktreePath ||
      isFolder ||
      !isBranchVisible ||
      !isGitHistoryExpanded ||
      !isGitHistoryVisible
    ) {
      return
    }

    const worktreeId = activeWorktreeId
    const requestId = gitHistoryRequestSeqRef.current + 1
    gitHistoryRequestSeqRef.current = requestId
    gitHistoryRequestByWorktreeRef.current[worktreeId] = requestId
    setGitHistoryByWorktree((prev) => {
      const previous = prev[worktreeId]
      return {
        ...prev,
        [worktreeId]: previous?.result
          ? { status: 'refreshing', result: previous.result }
          : { status: 'loading' }
      }
    })

    try {
      const connectionId = getConnectionId(worktreeId) ?? undefined
      const result = await getRuntimeGitHistory(
        {
          // Why: route the history read by the repo OWNER host, not the focused runtime.
          settings: activeRepoSettings,
          worktreeId,
          worktreePath,
          connectionId
        },
        { limit: 50, baseRef: compareBaseRef }
      )
      if (gitHistoryRequestByWorktreeRef.current[worktreeId] !== requestId) {
        return
      }
      setGitHistoryByWorktree((prev) => ({ ...prev, [worktreeId]: { status: 'ready', result } }))
    } catch (error) {
      if (gitHistoryRequestByWorktreeRef.current[worktreeId] !== requestId) {
        return
      }
      const message = error instanceof Error ? error.message : 'Failed to load commits'
      setGitHistoryByWorktree((prev) => {
        const previous = prev[worktreeId]
        return {
          ...prev,
          [worktreeId]: previous?.result
            ? { status: 'error', result: previous.result, error: message }
            : { status: 'error', error: message }
        }
      })
    }
  }, [
    activeRepoSettings,
    activeWorktreeId,
    compareBaseRef,
    isBranchVisible,
    isFolder,
    isGitHistoryExpanded,
    isGitHistoryVisible,
    worktreePath
  ])

  const refreshGitHistoryRef = useRef(refreshGitHistory)
  refreshGitHistoryRef.current = refreshGitHistory

  useEffect(() => {
    // Why: history shells out to git. Defer the first load until the user
    // expands Commits so source control stays cheap for large/remote repos.
    if (!isBranchVisible || !isGitHistoryExpanded || !isGitHistoryVisible) {
      return
    }
    void refreshGitHistoryRef.current()
  }, [
    // Why: history is fetched with compareBaseRef, so re-run when the upstream
    // compare base changes — effectiveBaseRef can stay put while it moves.
    activeWorktreeId,
    compareBaseRef,
    isBranchVisible,
    isFolder,
    isGitHistoryExpanded,
    isGitHistoryVisible,
    worktreePath
  ])

  // Why: stable-identity wrapper (not the raw useCallback above, whose
  // identity churns on every relevant dependency change) so the JSX refresh
  // button and callers outside this hook's own deps arrays always invoke the
  // latest implementation.
  const refreshGitHistoryStable = useCallback(
    (): Promise<void> => refreshGitHistoryRef.current(),
    []
  )

  return {
    refreshGitHistory: refreshGitHistoryStable,
    gitHistoryState,
    gitHistoryByWorktree
  }
}
