import { useCallback, useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { installWindowVisibilityInterval } from '@/lib/window-visibility-interval'
import { getConnectionId } from '@/lib/connection-context'
import { getRuntimeGitBranchCompare } from '@/runtime/runtime-git-client'
import {
  BRANCH_REFRESH_INTERVAL_MS,
  shouldClearBranchCompareForMissingBase
} from './source-control-helpers'
import {
  shouldRefreshBranchCompareForRemoteStatus,
  shouldRefreshBranchCompareForStatusHead,
  type BranchCompareRemoteStatusSnapshot,
  type BranchCompareStatusHeadSnapshot
} from './source-control-compare-summary'
import {
  useSourceControlGitHistoryRefresh,
  type UseSourceControlGitHistoryRefreshInput
} from './use-source-control-git-history-refresh'
import type {
  GitBranchChangeEntry,
  GitBranchCompareSummary,
  GitPushTarget,
  GlobalSettings
} from '../../../../shared/types'
import type { GitUpstreamStatus } from '../../../../shared/git-status-types'

export type UseSourceControlBranchCompareRefreshInput = Pick<
  UseSourceControlGitHistoryRefreshInput,
  'isGitHistoryExpanded' | 'isGitHistoryVisible' | 'worktreeMap'
> & {
  activeWorktreeId: string | null
  worktreePath: string | null
  isFolder: boolean
  isBranchVisible: boolean
  compareBaseRef: string | null
  activeRepoSettings: GlobalSettings | null
  branchName: string
  remoteStatus: GitUpstreamStatus | undefined
  activeGitStatusHead: string | null
  activeWorktreePushTarget: GitPushTarget | undefined
  beginGitBranchCompareRequest: (
    worktreeId: string,
    requestKey: string,
    baseRef: string,
    options?: { preserveExistingSummary?: boolean }
  ) => void
  setGitBranchCompareResult: (
    worktreeId: string,
    requestKey: string,
    result: { summary: GitBranchCompareSummary; entries: GitBranchChangeEntry[] }
  ) => void
  clearGitBranchCompare: (worktreeId: string) => void
  fetchUpstreamStatus: (
    worktreeId: string,
    worktreePath: string,
    connectionId: string | undefined,
    pushTarget: GitPushTarget | undefined,
    options: { runtimeTargetSettings: GlobalSettings | null }
  ) => Promise<GitUpstreamStatus | null>
}

/**
 * Branch-compare refresh + git-history refresh scheduling for
 * `SourceControlInner`. Extracted verbatim — see TASK-BIGFILE-226 (git-
 * history itself lives in a sibling hook this one composes, split out only
 * to stay under the 300-line file cap). The ref-indirection pattern
 * (`refreshBranchCompareRef`) is kept internally so recursive reruns and
 * effects always call the latest closure; `refreshBranchCompare` is returned
 * as a stable wrapper around that ref so external callers that intentionally
 * omit it from a `useCallback` deps array (to dodge unrelated re-creation)
 * still always invoke the current implementation.
 */
export function useSourceControlBranchCompareRefresh(
  input: UseSourceControlBranchCompareRefreshInput
) {
  const {
    activeWorktreeId,
    worktreePath,
    isFolder,
    isBranchVisible,
    compareBaseRef,
    activeRepoSettings,
    branchName,
    remoteStatus,
    activeGitStatusHead,
    isGitHistoryExpanded,
    isGitHistoryVisible,
    activeWorktreePushTarget,
    worktreeMap,
    beginGitBranchCompareRequest,
    setGitBranchCompareResult,
    clearGitBranchCompare,
    fetchUpstreamStatus
  } = input

  const gitHistory = useSourceControlGitHistoryRefresh({
    activeWorktreeId,
    worktreePath,
    isFolder,
    isBranchVisible,
    compareBaseRef,
    activeRepoSettings,
    isGitHistoryExpanded,
    isGitHistoryVisible,
    worktreeMap
  })

  const branchCompareInFlightRef = useRef(false)
  const branchCompareRerunRef = useRef(false)
  const branchCompareRunPromiseRef = useRef<Promise<void> | null>(null)
  const refreshBranchCompareRef = useRef<() => Promise<void>>(async () => {})
  const branchCompareStatusHeadRef = useRef<BranchCompareStatusHeadSnapshot | null>(null)
  const branchCompareRemoteStatusRef = useRef<BranchCompareRemoteStatusSnapshot | null>(null)

  const runBranchCompare = useCallback(async () => {
    if (!activeWorktreeId || !worktreePath || !compareBaseRef || isFolder) {
      return
    }

    const requestKey = `${activeWorktreeId}:${compareBaseRef}:${Date.now()}`
    const existingSummary =
      useAppStore.getState().gitBranchCompareSummaryByWorktree[activeWorktreeId]

    // Why: only show the loading spinner for the very first branch compare
    // request, or when the base ref has changed (user picked a new one, or
    // getBaseRefDefault corrected a stale cross-repo value).  Polling retries
    // — whether the previous result was 'ready' *or* an error — keep the
    // current UI visible until the new IPC result arrives.  Resetting to
    // 'loading' on every poll when the compare is in an error state caused a
    // visible loading→error→loading→error flicker.
    const baseRefChanged = existingSummary && existingSummary.baseRef !== compareBaseRef
    const shouldResetToLoading = !existingSummary || baseRefChanged
    if (shouldResetToLoading) {
      beginGitBranchCompareRequest(activeWorktreeId, requestKey, compareBaseRef)
    } else {
      beginGitBranchCompareRequest(activeWorktreeId, requestKey, compareBaseRef, {
        preserveExistingSummary: true
      })
    }

    try {
      const connectionId = getConnectionId(activeWorktreeId ?? null) ?? undefined
      const result = await getRuntimeGitBranchCompare(
        {
          // Why: route the branch compare by the repo OWNER host, not the focused runtime.
          settings: activeRepoSettings,
          worktreeId: activeWorktreeId,
          worktreePath,
          connectionId
        },
        compareBaseRef
      )
      setGitBranchCompareResult(activeWorktreeId, requestKey, result)
    } catch (error) {
      setGitBranchCompareResult(activeWorktreeId, requestKey, {
        summary: {
          baseRef: compareBaseRef,
          baseOid: null,
          compareRef: branchName,
          headOid: null,
          mergeBase: null,
          changedFiles: 0,
          status: 'error',
          errorMessage: error instanceof Error ? error.message : 'Branch compare failed'
        },
        entries: []
      })
    }
  }, [
    activeRepoSettings,
    activeWorktreeId,
    beginGitBranchCompareRequest,
    branchName,
    compareBaseRef,
    isFolder,
    setGitBranchCompareResult,
    worktreePath
  ])

  const refreshBranchCompare = useCallback(async () => {
    if (branchCompareInFlightRef.current) {
      branchCompareRerunRef.current = true
      return branchCompareRunPromiseRef.current ?? undefined
    }

    branchCompareInFlightRef.current = true
    const runPromise = (async (): Promise<void> => {
      // Why: branch compare shells out to git from both event-driven refreshes
      // and the fallback timer. Keep one compare chain in flight and
      // collapse skipped ticks into one trailing refresh instead of stacking
      // subprocesses while preserving the await contract for direct callers.
      try {
        await runBranchCompare()
      } finally {
        branchCompareInFlightRef.current = false
        if (branchCompareRerunRef.current) {
          branchCompareRerunRef.current = false
          await refreshBranchCompareRef.current()
        }
      }
    })()
    branchCompareRunPromiseRef.current = runPromise
    try {
      await runPromise
    } finally {
      if (branchCompareRunPromiseRef.current === runPromise) {
        branchCompareRunPromiseRef.current = null
      }
    }
  }, [runBranchCompare])

  refreshBranchCompareRef.current = refreshBranchCompare

  useEffect(() => {
    if (!activeWorktreeId || !worktreePath || !isBranchVisible || !compareBaseRef || isFolder) {
      branchCompareStatusHeadRef.current = null
      return
    }

    const current = {
      baseRef: compareBaseRef,
      statusHead: activeGitStatusHead,
      worktreeId: activeWorktreeId
    }
    const previous = branchCompareStatusHeadRef.current
    branchCompareStatusHeadRef.current = current
    if (shouldRefreshBranchCompareForStatusHead(previous, current)) {
      void refreshBranchCompareRef.current()
    }
  }, [
    activeGitStatusHead,
    activeWorktreeId,
    compareBaseRef,
    isBranchVisible,
    isFolder,
    worktreePath
  ])

  useEffect(() => {
    if (!activeWorktreeId || !worktreePath || !isBranchVisible || !compareBaseRef || isFolder) {
      branchCompareRemoteStatusRef.current = null
      return
    }

    // Why: pushing a branch can move its remote-tracking base and ahead count
    // without changing local HEAD, so the HEAD-change effect alone misses it.
    const current = {
      ahead: remoteStatus?.ahead ?? null,
      baseRef: compareBaseRef,
      behind: remoteStatus?.behind ?? null,
      hasUpstream: remoteStatus?.hasUpstream ?? null,
      upstreamName: remoteStatus?.upstreamName ?? null,
      worktreeId: activeWorktreeId
    }
    const previous = branchCompareRemoteStatusRef.current
    branchCompareRemoteStatusRef.current = current
    if (shouldRefreshBranchCompareForRemoteStatus(previous, current)) {
      void refreshBranchCompareRef.current()
    }
  }, [
    activeWorktreeId,
    compareBaseRef,
    isBranchVisible,
    isFolder,
    remoteStatus?.ahead,
    remoteStatus?.behind,
    remoteStatus?.hasUpstream,
    remoteStatus?.upstreamName,
    worktreePath
  ])

  useEffect(() => {
    if (!activeWorktreeId || !worktreePath || !isBranchVisible || !compareBaseRef || isFolder) {
      return
    }

    // Why: git-status HEAD changes refresh branch compare immediately. Keep a
    // visible-window fallback for base refs or remote updates that do not move HEAD.
    return installWindowVisibilityInterval({
      run: () => void refreshBranchCompareRef.current(),
      intervalMs: BRANCH_REFRESH_INTERVAL_MS
    })
  }, [activeWorktreeId, compareBaseRef, isBranchVisible, isFolder, worktreePath])

  useEffect(() => {
    // Why: when the compare-base policy resolves to no base, runBranchCompare
    // bails out; drop any stale summary so the
    // committed-changes section and "vs" row disappear and only the working tree
    // shows. Wait until upstream status has loaded so the summary doesn't flicker.
    if (
      !activeWorktreeId ||
      !shouldClearBranchCompareForMissingBase({ isFolder, compareBaseRef, remoteStatus })
    ) {
      return
    }
    clearGitBranchCompare(activeWorktreeId)
  }, [activeWorktreeId, clearGitBranchCompare, compareBaseRef, isFolder, remoteStatus])

  useEffect(() => {
    // Why: gate on isBranchVisible so we don't spawn git processes while the
    // sidebar is closed. Store-slice remote operations refresh upstream-status
    // on success anyway, so the user's first sidebar open will show accurate
    // state.
    if (!activeWorktreeId || !worktreePath || isFolder || !isBranchVisible) {
      return
    }
    const connectionId = getConnectionId(activeWorktreeId) ?? undefined
    void fetchUpstreamStatus(
      activeWorktreeId,
      worktreePath,
      connectionId,
      activeWorktreePushTarget,
      {
        runtimeTargetSettings: activeRepoSettings
      }
    )
  }, [
    activeRepoSettings,
    activeWorktreePushTarget,
    activeWorktreeId,
    fetchUpstreamStatus,
    isBranchVisible,
    isFolder,
    worktreePath
  ])

  // Why: exposed as a stable-identity wrapper (not the raw useCallback above,
  // whose identity churns on every branch-compare-relevant dependency
  // change) so external callers — the post-commit/post-remote-action refresh
  // sites — can invoke the latest implementation without listing it in their
  // own deps array. Mirrors the ref indirection this cluster already relied
  // on before extraction.
  const refreshBranchCompareStable = useCallback(
    (): Promise<void> => refreshBranchCompareRef.current(),
    []
  )

  return {
    refreshBranchCompare: refreshBranchCompareStable,
    refreshGitHistory: gitHistory.refreshGitHistory,
    gitHistoryState: gitHistory.gitHistoryState,
    gitHistoryByWorktree: gitHistory.gitHistoryByWorktree
  }
}
