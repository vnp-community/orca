/**
 * use-review-branch-compare-fetch.ts — FE-CV-TASK-051-01
 *
 * When Review opens before Source Control has produced a branch-compare summary, fetch one
 * through the runtime git client (local IPC, SSH or remote runtime) and store it with the same
 * actions Source Control uses. No new git command: it is the existing `git.branchCompare`.
 *
 * @module components/review-map/shell/use-review-branch-compare-fetch
 */

import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { getConnectionId } from '@/lib/connection-context'
import { getRepoOwnerRoutedSettings } from '@/lib/repo-runtime-owner'
import { getRuntimeGitBranchCompare } from '@/runtime/runtime-git-client'

/** Base the compare runs against: the worktree's pinned base, else the repo's. */
export function reviewCompareBaseRef(
  worktree: { baseRef?: string | null } | null | undefined,
  repo: { worktreeBaseRef?: string | null } | null | undefined
): string | null {
  return worktree?.baseRef?.trim() || repo?.worktreeBaseRef?.trim() || null
}

export function useReviewBranchCompareFetch(worktreeId: string, enabled: boolean): void {
  const hasSummary = useAppStore((s) => Boolean(s.gitBranchCompareSummaryByWorktree[worktreeId]))
  // One attempt per worktree+base per mount; Source Control's poller owns refreshes after that.
  const attemptedRef = useRef<string | null>(null)

  useEffect(() => {
    if (!enabled || hasSummary) {
      return
    }
    const state = useAppStore.getState()
    const worktree = Object.values(state.worktreesByRepo)
      .flat()
      .find((w) => w.id === worktreeId)
    if (!worktree?.path) {
      return
    }
    const repo = state.repos.find((r) => r.id === worktree.repoId) ?? null
    const baseRef = reviewCompareBaseRef(worktree, repo)
    const attemptKey = baseRef ? `${worktreeId}:${baseRef}` : null
    if (!baseRef || attemptedRef.current === attemptKey) {
      return
    }
    attemptedRef.current = attemptKey
    const requestKey = `review:${worktreeId}:${baseRef}:${Date.now()}`
    // Why: no 'loading' placeholder; the worktree's pinned base keeps the default scope usable meanwhile.
    state.beginGitBranchCompareRequest(worktreeId, requestKey, baseRef, {
      preserveExistingSummary: true
    })
    void getRuntimeGitBranchCompare(
      {
        // Route by the repo owner host, as Source Control does.
        settings: getRepoOwnerRoutedSettings(state.settings, repo),
        worktreeId,
        worktreePath: worktree.path,
        connectionId: getConnectionId(worktreeId) ?? undefined
      },
      baseRef
    )
      .then((result) =>
        useAppStore.getState().setGitBranchCompareResult(worktreeId, requestKey, result)
      )
      .catch(() => {
        // An error summary would replace the usable fallback scope; Source Control reports git errors.
      })
  }, [enabled, hasSummary, worktreeId])
}
