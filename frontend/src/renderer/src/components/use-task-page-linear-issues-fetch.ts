import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { reconcileTaskPageLinearIssuesAfterLandingRefresh } from '@/components/task-page-cache-selectors'
import {
  buildLinearIssueListReadArgs,
  buildLinearIssueListRequestSignature,
  shouldForceLinearIssueListRead
} from '@/components/task-page-linear-issue-request'
import {
  LINEAR_ISSUE_LIST_MAX,
  clampLinearIssueListLimit
} from '../../../shared/linear-issue-read-limits'
import { linearIssueAttributeFilterSignature } from '../../../shared/linear-issue-attribute-filter'
import type { LinearIssueAttributeFilter } from '../../../shared/linear-issue-attribute-filter'
import type { LinearCollectionResult, LinearIssue, TaskProvider } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of TaskPage.tsx (TASK-BIGFILE-241) to stay under the
// 300-line file budget — the plain "all issues" Linear list fetch, the
// densest single effect in the Linear browse domain (search vs. attribute
// filter reads, landing-refresh probe, and stale-request guarding). Pure
// effect hook — owns no state beyond its private request-tracking refs.
const LINEAR_ITEM_LIMIT = 36

type UseTaskPageLinearIssuesFetchParams = {
  taskResumeApplied: boolean
  taskSource: TaskProvider
  linearMode: string
  linearConnected: boolean
  selectedLinearWorkspaceId: string | null
  appliedLinearSearch: string
  linearIssueLimit: number
  linearRefreshNonce: number
  linearAttributeFilter: LinearIssueAttributeFilter
  linearListInvalidationVersionForSource: number
  linearTaskSourceContext: TaskSourceContext | null | undefined
  setLinearError: (value: string | null) => void
  setLinearIssuesHasMore: (value: boolean) => void
  setLinearIssues: (updater: LinearIssue[] | ((current: LinearIssue[]) => LinearIssue[])) => void
  setLinearLoading: (value: boolean) => void
}

export function useTaskPageLinearIssuesFetch({
  taskResumeApplied,
  taskSource,
  linearMode,
  linearConnected,
  selectedLinearWorkspaceId,
  appliedLinearSearch,
  linearIssueLimit,
  linearRefreshNonce,
  linearAttributeFilter,
  linearListInvalidationVersionForSource,
  linearTaskSourceContext,
  setLinearError,
  setLinearIssuesHasMore,
  setLinearIssues,
  setLinearLoading
}: UseTaskPageLinearIssuesFetchParams): void {
  const getCachedLinearIssues = useAppStore((s) => s.getCachedLinearIssues)
  const listLinearIssues = useAppStore((s) => s.listLinearIssues)
  const searchLinearIssues = useAppStore((s) => s.searchLinearIssues)

  const linearAttributeFilterSignatureRef = useRef(
    linearIssueAttributeFilterSignature(linearAttributeFilter)
  )
  const lastLinearRequestRef = useRef<{ nonce: number; signature: string } | null>(null)
  const landingLinearRefreshKeysRef = useRef<ReadonlySet<string>>(new Set())

  // Why: fetch Linear issues when the tab is active and connected. Empty search
  // uses the plain `all` list with optional server-side attribute filters.
  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (taskSource !== 'linear') {
      return
    }
    if (linearMode !== 'issues') {
      return
    }
    if (!linearConnected) {
      return
    }

    let cancelled = false
    setLinearError(null)

    const trimmed = appliedLinearSearch.trim()
    const effectiveLinearIssueLimit = clampLinearIssueListLimit(linearIssueLimit)
    const searchActive = trimmed.length > 0
    const listReadArgs = buildLinearIssueListReadArgs({
      filter: 'all',
      limit: effectiveLinearIssueLimit,
      attributeFilter: linearAttributeFilter,
      searchActive,
      allowAttributeFilter: selectedLinearWorkspaceId !== 'all'
    })
    const readArgs = searchActive
      ? ({ kind: 'search', query: trimmed, limit: LINEAR_ITEM_LIMIT } as const)
      : listReadArgs
    const cachedResult = getCachedLinearIssues(readArgs, { sourceContext: linearTaskSourceContext })
    if (readArgs.kind === 'search') {
      setLinearIssuesHasMore(false)
      if (cachedResult) {
        setLinearIssues(cachedResult as LinearIssue[])
      }
    } else if (cachedResult) {
      const collection = cachedResult as LinearCollectionResult<LinearIssue>
      setLinearIssues(collection.items)
      setLinearIssuesHasMore(
        Boolean(collection.hasMore) && effectiveLinearIssueLimit < LINEAR_ISSUE_LIST_MAX
      )
    }

    const nextFilterSignature = linearIssueAttributeFilterSignature(linearAttributeFilter)
    const previousFilterSignature = linearAttributeFilterSignatureRef.current
    linearAttributeFilterSignatureRef.current = nextFilterSignature
    const filterForce = shouldForceLinearIssueListRead({
      previousFilterSignature,
      nextFilterSignature,
      refreshForced: false
    })

    const requestSignature = buildLinearIssueListRequestSignature({
      sourceContext: linearTaskSourceContext,
      workspaceId: selectedLinearWorkspaceId,
      filter: 'all',
      limit: effectiveLinearIssueLimit,
      attributeFilter: linearAttributeFilter,
      searchQuery: searchActive ? trimmed : undefined
    })
    const previousRequest = lastLinearRequestRef.current
    const forceRefresh =
      filterForce ||
      (linearRefreshNonce > 0 &&
        previousRequest?.nonce !== linearRefreshNonce &&
        previousRequest?.signature === requestSignature)
    lastLinearRequestRef.current = { nonce: linearRefreshNonce, signature: requestSignature }
    const shouldProbeOnLanding =
      !forceRefresh &&
      cachedResult !== null &&
      !landingLinearRefreshKeysRef.current.has(requestSignature)
    if (shouldProbeOnLanding) {
      landingLinearRefreshKeysRef.current = new Set([
        ...landingLinearRefreshKeysRef.current,
        requestSignature
      ])
    }

    // Why: cached rows should remain visible on navigation. Only an explicit
    // refresh or a true cache miss needs the blocking loading state.
    setLinearLoading(forceRefresh || cachedResult === null)

    const request =
      readArgs.kind === 'search'
        ? searchLinearIssues(readArgs.query, LINEAR_ITEM_LIMIT, {
            force: forceRefresh || shouldProbeOnLanding,
            sourceContext: linearTaskSourceContext
          })
        : listLinearIssues(listReadArgs, {
            force: forceRefresh || shouldProbeOnLanding,
            sourceContext: linearTaskSourceContext
          })

    void request
      .then((result) => {
        if (
          cancelled ||
          lastLinearRequestRef.current?.signature !== requestSignature ||
          lastLinearRequestRef.current?.nonce !== linearRefreshNonce
        ) {
          return
        }
        if (readArgs.kind === 'search') {
          const issues = result as LinearIssue[]
          setLinearIssuesHasMore(false)
          if (shouldProbeOnLanding) {
            setLinearIssues((current) =>
              reconcileTaskPageLinearIssuesAfterLandingRefresh(current, issues)
            )
          } else {
            setLinearIssues(issues)
          }
        } else {
          const collection = result as LinearCollectionResult<LinearIssue>
          setLinearIssuesHasMore(
            Boolean(collection.hasMore) && effectiveLinearIssueLimit < LINEAR_ISSUE_LIST_MAX
          )
          setLinearIssues((current) =>
            shouldProbeOnLanding
              ? reconcileTaskPageLinearIssuesAfterLandingRefresh(current, collection.items)
              : collection.items
          )
        }
        setLinearLoading(false)
      })
      .catch((err) => {
        if (
          cancelled ||
          lastLinearRequestRef.current?.signature !== requestSignature ||
          lastLinearRequestRef.current?.nonce !== linearRefreshNonce
        ) {
          return
        }
        setLinearError(err instanceof Error ? err.message : 'Failed to load Linear issues.')
        setLinearLoading(false)
      })

    return () => {
      cancelled = true
    }
    // Why: searchLinearIssues and listLinearIssues are stable zustand selectors;
    // depending on them would re-run the effect on unrelated store updates.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    taskSource,
    linearMode,
    linearConnected,
    selectedLinearWorkspaceId,
    appliedLinearSearch,
    linearIssueLimit,
    linearRefreshNonce,
    linearAttributeFilter,
    linearListInvalidationVersionForSource,
    taskResumeApplied,
    getCachedLinearIssues,
    linearTaskSourceContext
  ])
}
