import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { stripRepoQualifiers } from '../../../shared/task-query'
import { PER_REPO_FETCH_LIMIT, CROSS_REPO_DISPLAY_LIMIT } from '@/lib/new-workspace'
import {
  deriveTaskPageGitHubWorkItemsFetchOptions,
  reconcileTaskPagePagesAfterLandingRefresh,
  shouldResetTaskPagePaginationAfterLandingRefresh,
  shouldReplaceTaskPageItemsAfterRefresh
} from '@/components/task-page-cache-selectors'
import { scopeGitHubTaskSearch } from '@/components/task-page-github-task-kind'
import type { GitHubTaskKind } from '@/components/task-page-localized-options'
import type { GitHubWorkItem, Repo, TaskProvider, TaskViewPresetId } from '../../../shared/types'
import type { TaskSourceContext } from '../../../shared/task-source-context'

// Why: split out of use-task-page-github-state.ts (TASK-BIGFILE-236) to stay
// under the 300-line file budget — this is the GitHub work-items data-fetch
// slice: search-box debounce, resume-state persistence, and the main
// cross-repo fetch effect. Pure effects hook — owns no state of its own,
// only refs private to its own dedupe/landing-probe bookkeeping.
const TASK_SEARCH_DEBOUNCE_MS = 300

type UseTaskPageGitHubWorkItemsFetchParams = {
  taskSource: TaskProvider
  taskResumeApplied: boolean
  githubMode: 'items' | 'project'
  selectedRepos: readonly Repo[]
  taskSearchInput: string
  appliedTaskSearch: string
  activeTaskPreset: TaskViewPresetId | null
  activeGithubTaskKind: GitHubTaskKind
  taskRefreshNonce: number
  retryingSourceKeys: ReadonlySet<string>
  setAppliedTaskSearch: (value: string) => void
  setTasksFiltering: (value: boolean) => void
  setTasksLoading: (value: boolean) => void
  setTasksRefreshing: (value: boolean) => void
  setTasksError: (value: string | null) => void
  setFailedCount: (value: number) => void
  setPages: React.Dispatch<React.SetStateAction<GitHubWorkItem[][]>>
  setCurrentPage: (value: number) => void
  setTotalItemCount: (value: number | null) => void
  setRetryingSourceKeys: React.Dispatch<React.SetStateAction<ReadonlySet<string>>>
  getTaskPageRepoSourceContext: (
    repo: Repo | null | undefined,
    provider: 'github' | 'gitlab'
  ) => TaskSourceContext | null
}

export function useTaskPageGitHubWorkItemsFetch({
  taskSource,
  taskResumeApplied,
  githubMode,
  selectedRepos,
  taskSearchInput,
  appliedTaskSearch,
  activeTaskPreset,
  activeGithubTaskKind,
  taskRefreshNonce,
  retryingSourceKeys,
  setAppliedTaskSearch,
  setTasksFiltering,
  setTasksLoading,
  setTasksRefreshing,
  setTasksError,
  setFailedCount,
  setPages,
  setCurrentPage,
  setTotalItemCount,
  setRetryingSourceKeys,
  getTaskPageRepoSourceContext
}: UseTaskPageGitHubWorkItemsFetchParams): void {
  const getCachedWorkItems = useAppStore((s) => s.getCachedWorkItems)
  const fetchWorkItemsAcrossRepos = useAppStore((s) => s.fetchWorkItemsAcrossRepos)
  const countWorkItemsAcrossRepos = useAppStore((s) => s.countWorkItemsAcrossRepos)
  const workItemsInvalidationNonce = useAppStore((s) => s.workItemsInvalidationNonce)
  const setTaskResumeState = useAppStore((s) => s.setTaskResumeState)

  // Why: the fetch effect uses this to detect when a nonce bump is from the
  // user clicking the refresh button (force=true) vs. re-running for any
  // other reason — e.g. a repo change while the nonce happens to be > 0.
  const lastFetchedNonceRef = useRef(-1)
  // Why: analogous to `lastFetchedNonceRef` for the invalidation nonce. A
  // preference flip should force the dispatch past fetch-dedupe (same repos +
  // same query, cache just evicted — without `force: true` the fan-out could
  // collapse onto a stale in-flight request that resolved against the
  // pre-flip source).
  const lastFetchedInvalidationNonceRef = useRef(0)
  // Why: entering Tasks with fresh cache should still verify remote status
  // once, but the result is reconciled into existing rows to avoid a full
  // table shuffle when only status/key fields changed.
  const landingGitHubRefreshKeysRef = useRef<ReadonlySet<string>>(new Set())

  // Why: session-only search-box debounce. Persisting the applied query is a
  // separate concern (below) so a fast typist doesn't spam setTaskResumeState.
  const githubSearchPersistReadyRef = useRef(false)

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    const timeout = window.setTimeout(() => {
      const scoped = scopeGitHubTaskSearch(taskSearchInput, activeGithubTaskKind)
      if (scoped !== appliedTaskSearch) {
        setTasksFiltering(true)
      }
      setAppliedTaskSearch(scoped)
    }, TASK_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timeout)
  }, [
    activeGithubTaskKind,
    appliedTaskSearch,
    setAppliedTaskSearch,
    setTasksFiltering,
    taskSearchInput,
    taskResumeApplied
  ])

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    if (!githubSearchPersistReadyRef.current) {
      githubSearchPersistReadyRef.current = true
      return
    }
    // Why: persist the debounced applied query regardless of the active
    // preset. The preset-click handler writes the canonical query for that
    // preset, so persisting again here is at worst idempotent. When the
    // user types into the search box `handleTaskSearchChange` clears the
    // preset, but persisting unconditionally also covers paths that change
    // appliedTaskSearch without going through that handler.
    setTaskResumeState({
      githubItemsPreset: activeTaskPreset,
      githubItemsQuery: appliedTaskSearch.trim()
    })
  }, [activeTaskPreset, appliedTaskSearch, setTaskResumeState, taskResumeApplied])

  useEffect(() => {
    if (!taskResumeApplied) {
      return
    }
    // Why: both early-return branches must clear `retryingSourceKeys` — if the
    // user clicks Retry and then switches `taskSource` away from 'github' (or
    // somehow ends up with zero repos selected) before the fetch dispatches,
    // neither the `.then` nor the `.catch` below will fire, and the Retry
    // button would stay stuck in its disabled/Retrying state indefinitely.
    if (taskSource !== 'github' || githubMode !== 'items') {
      setRetryingSourceKeys(new Set())
      setTasksRefreshing(false)
      setTasksFiltering(false)
      return
    }
    if (selectedRepos.length === 0) {
      setRetryingSourceKeys(new Set())
      setTasksRefreshing(false)
      setTasksFiltering(false)
      return
    } // unreachable — multi-combobox forbids empty

    // Why: `repo:owner/name` qualifiers are silently dropped before fan-out
    // because in cross-repo mode they would pin every per-repo fetch to a
    // single repo and zero out the rest. See stripRepoQualifiers.
    const q = stripRepoQualifiers(appliedTaskSearch.trim())
    let cancelled = false

    // Why: paint cached rows synchronously before awaiting the fan-out so
    // selection changes don't leave the previous selection's rows on screen
    // for a frame. Any repo without a cache entry simply contributes nothing
    // to this pre-paint; the fetch will fill it in.
    const preMerged: GitHubWorkItem[] = []
    let anyUncached = false
    let anyRepoCached = false
    for (const r of selectedRepos) {
      const cached = getCachedWorkItems(
        r.id,
        PER_REPO_FETCH_LIMIT,
        q,
        r.path,
        getTaskPageRepoSourceContext(r, 'github')
      )
      if (cached === null) {
        anyUncached = true
      } else {
        anyRepoCached = true
        preMerged.push(...cached)
      }
    }
    // Why: always replace — if preMerged is empty (e.g. query just changed and
    // no repo has a cache entry for it), we clear the previous query's rows
    // rather than leaving them on screen under the spinner.
    const page0 =
      preMerged.length > 0
        ? [...preMerged]
            .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
            .slice(0, CROSS_REPO_DISPLAY_LIMIT)
        : []
    setPages([page0])
    setCurrentPage(0)
    setTotalItemCount(null)
    setTasksError(null)
    setFailedCount(0) // reset so a prior failure banner doesn't linger
    setTasksLoading(anyUncached)

    // Preserve the existing nonce-gated force behavior.
    const forceRefresh = taskRefreshNonce !== lastFetchedNonceRef.current
    lastFetchedNonceRef.current = taskRefreshNonce
    // Why: a preference flip bumps `workItemsInvalidationNonce`. Treat that
    // bump as a forced refresh so the fan-out bypasses the in-flight dedupe
    // map — otherwise an overlapping request started before the flip could
    // resolve the new fetch and repopulate the cache with pre-flip data.
    const preferenceInvalidated =
      workItemsInvalidationNonce !== lastFetchedInvalidationNonceRef.current
    lastFetchedInvalidationNonceRef.current = workItemsInvalidationNonce
    const forcedFetch = (forceRefresh && taskRefreshNonce > 0) || preferenceInvalidated
    const repoArgs = selectedRepos.map((r) => ({
      repoId: r.id,
      path: r.path,
      executionHostId: r.executionHostId,
      sourceContext: getTaskPageRepoSourceContext(r, 'github')
    }))
    const landingRefreshKey = `${repoArgs.map((r) => `${r.repoId}:${r.path}`).join('|')}::${q}`
    const shouldProbeOnLanding =
      !forcedFetch && anyRepoCached && !landingGitHubRefreshKeysRef.current.has(landingRefreshKey)
    if (shouldProbeOnLanding) {
      landingGitHubRefreshKeysRef.current = new Set([
        ...landingGitHubRefreshKeysRef.current,
        landingRefreshKey
      ])
    }
    // Why: manual refresh keeps cached rows visible, so the normal
    // `tasksLoading` flag may stay false. Track the forced fetch separately
    // so the toolbar still shows a refresh-in-progress affordance.
    setTasksRefreshing(forcedFetch)

    // Why: snapshot the retrying source keys at effect-dispatch so overlapping
    // retries don't clear each other's pending state. An earlier cancelled
    // effect settling after a newer retry starts would otherwise wipe the
    // newer retry's source from the set. Clearing only the keys captured
    // when this effect dispatched preserves later additions.
    const dispatchedRetrySourceKeys = retryingSourceKeys
    void fetchWorkItemsAcrossRepos(repoArgs, PER_REPO_FETCH_LIMIT, CROSS_REPO_DISPLAY_LIMIT, q, {
      ...deriveTaskPageGitHubWorkItemsFetchOptions(forcedFetch, shouldProbeOnLanding)
    })
      .then(({ items, failedCount: failed }) => {
        // Why: clear only the sources this effect was responsible for
        // retrying (the snapshot captured at dispatch time). Overlapping
        // retries — a second click while a prior fetch is still in flight
        // — must not clear the newer source from the set, so we can't just
        // reset the whole set here. The early-return branches above reset
        // the whole set because those branches won't dispatch a fetch.
        setRetryingSourceKeys((prev) => {
          if (dispatchedRetrySourceKeys.size === 0) {
            return prev
          }
          const next = new Set(prev)
          for (const key of dispatchedRetrySourceKeys) {
            next.delete(key)
          }
          return next
        })
        if (cancelled) {
          return
        }
        if (shouldProbeOnLanding) {
          const replaceFirstPage = shouldReplaceTaskPageItemsAfterRefresh(page0, items)
          const resetPagination = shouldResetTaskPagePaginationAfterLandingRefresh(page0, items)
          setPages((current) => reconcileTaskPagePagesAfterLandingRefresh(current, items))
          if (replaceFirstPage || resetPagination) {
            setCurrentPage(0)
          }
        } else {
          setPages([items])
          setCurrentPage(0)
        }
        setFailedCount(failed)
        setTasksLoading(false)
        setTasksRefreshing(false)
        setTasksFiltering(false)
      })
      .catch((err) => {
        // Why: fetchWorkItemsAcrossRepos swallows per-repo failures, so a
        // reject here means an IPC-level or programmer error — surface it.
        // Clear only the sources this effect was responsible for retrying
        // (the snapshot captured at dispatch time). Overlapping retries —
        // a second click while a prior fetch is still in flight — must
        // not clear the newer source from the set, so we can't just reset
        // the whole set here. The early-return branches above reset the
        // whole set because those branches won't dispatch a fetch.
        setRetryingSourceKeys((prev) => {
          if (dispatchedRetrySourceKeys.size === 0) {
            return prev
          }
          const next = new Set(prev)
          for (const key of dispatchedRetrySourceKeys) {
            next.delete(key)
          }
          return next
        })
        if (cancelled) {
          return
        }
        setTasksError(err instanceof Error ? err.message : 'Failed to load GitHub work.')
        setFailedCount(0) // the per-repo banner would be misleading next to tasksError
        setTasksLoading(false)
        setTasksRefreshing(false)
        setTasksFiltering(false)
      })

    // Why: fire-and-forget count query in parallel with the items fetch.
    // The search API is cached 120s server-side so this doesn't add
    // meaningful latency or rate-limit pressure.
    void countWorkItemsAcrossRepos(
      selectedRepos.map((r) => ({
        repoId: r.id,
        path: r.path,
        executionHostId: r.executionHostId,
        sourceContext: getTaskPageRepoSourceContext(r, 'github')
      })),
      q
    ).then((count) => {
      if (!cancelled) {
        setTotalItemCount(count)
      }
    })

    return () => {
      cancelled = true
    }
    // Why: getCachedWorkItems and fetchWorkItemsAcrossRepos are stable zustand
    // selectors; depending on them would re-run the effect on unrelated store
    // updates. `workItemsInvalidationNonce` is explicitly included so a
    // preference flip (which only evicts cache) re-dispatches this effect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    selectedRepos,
    appliedTaskSearch,
    taskRefreshNonce,
    taskSource,
    githubMode,
    workItemsInvalidationNonce,
    taskResumeApplied
  ])
}
