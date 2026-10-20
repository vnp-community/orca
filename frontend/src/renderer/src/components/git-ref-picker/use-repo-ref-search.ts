import { useCallback, useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForRepo } from '@/lib/repo-runtime-owner'
import {
  getRuntimeRepoBaseRefDefault,
  searchRuntimeRepoBaseRefs
} from '@/runtime/runtime-repo-client'
import { isRuntimeRepoRefSearchQueryWithinLimit } from '@/runtime/runtime-repo-search-bounds'

export const REPO_REF_SEARCH_DEBOUNCE_MS = 200
export const REPO_REF_SEARCH_MIN_QUERY_LENGTH = 2
export const REPO_REF_SEARCH_RESULT_LIMIT = 20

/**
 * idle: nothing to search yet; searching: debounce or request in flight; results / empty: the host answered;
 * error: the search failed on the host; offline: the host could not be reached at all.
 */
export type RepoRefSearchStatus = 'idle' | 'searching' | 'results' | 'empty' | 'error' | 'offline'

/** ready with a null ref means the repo has no default; failed means the lookup itself failed. */
export type RepoRefDefaultStatus = 'loading' | 'ready' | 'failed'

// Failures of the transport (SSH host down, relay gone, no network) as opposed to a host that answered with an error.
const OFFLINE_ERROR_CODES: ReadonlySet<string> = new Set([
  'runtime_unavailable',
  'remote_runtime_unavailable',
  'connection_refused',
  'timeout'
])

export function isRepoRefHostOffline(error: unknown): boolean {
  if (typeof navigator !== 'undefined' && navigator.onLine === false) {
    return true
  }
  const code = (error as { code?: unknown } | null)?.code
  return typeof code === 'string' && OFFLINE_ERROR_CODES.has(code)
}

type DefaultsState = {
  scope: string
  defaultBaseRef: string | null
  remoteCount: number
  status: 'ready' | 'failed'
}

type SearchOutcome = {
  key: string
  status: 'results' | 'empty' | 'error' | 'offline'
  results: string[]
}

export type RepoRefSearch = {
  query: string
  setQuery: (query: string) => void
  results: readonly string[]
  status: RepoRefSearchStatus
  /** Never fabricated: null until the host answers, and when the repo has none (do not seed 'origin/main'). */
  defaultBaseRef: string | null
  /** 0 until the host answers and when it fails: a missing hint is better than a wrong one. */
  remoteCount: number
  defaultStatus: RepoRefDefaultStatus
  /** Runs the default lookup and the current search again, after an error or offline state. */
  retry: () => void
}

export type RepoRefSearchOptions = {
  /** False keeps the query but never asks the host to search (the caller already holds the whole list). Default true. */
  searchEnabled?: boolean
  /** False skips the default-branch lookup for a caller that already knows the default. Default true. */
  lookupDefault?: boolean
}

/**
 * Branch search and default-branch lookup of one repo, shared by every ref picker. All state is tagged with the repo and
 * runtime owner it was loaded for, so switching repo can never show the previous repo's default, query or results.
 */
export function useRepoRefSearch(
  repoId: string,
  { searchEnabled = true, lookupDefault = true }: RepoRefSearchOptions = {}
): RepoRefSearch {
  const activeRuntimeEnvironmentId = useAppStore((state) =>
    getRuntimeEnvironmentIdForRepo(state, repoId)
  )
  const scope = `${activeRuntimeEnvironmentId ?? ''}\u0000${repoId}`
  const [queryState, setQueryState] = useState<{ scope: string; text: string } | null>(null)
  const [defaults, setDefaults] = useState<DefaultsState | null>(null)
  const [outcome, setOutcome] = useState<SearchOutcome | null>(null)
  const [attempt, setAttempt] = useState(0)

  const query = queryState?.scope === scope ? queryState.text : ''
  const trimmedQuery = query.trim()
  const withinLimit = isRuntimeRepoRefSearchQueryWithinLimit(query)
  const searchable =
    searchEnabled && withinLimit && trimmedQuery.length >= REPO_REF_SEARCH_MIN_QUERY_LENGTH
  const searchKey = `${scope}\u0000${attempt}\u0000${trimmedQuery}`

  useEffect(() => {
    if (!lookupDefault) {
      return
    }
    let stale = false
    getRuntimeRepoBaseRefDefault({ activeRuntimeEnvironmentId }, repoId)
      .then((result) => {
        if (!stale) {
          setDefaults({
            scope,
            defaultBaseRef: result.defaultBaseRef,
            remoteCount: result.remoteCount,
            status: 'ready'
          })
        }
      })
      .catch((err: unknown) => {
        console.error('[RepoRefPicker] getBaseRefDefault failed', err)
        if (!stale) {
          setDefaults({ scope, defaultBaseRef: null, remoteCount: 0, status: 'failed' })
        }
      })
    return () => {
      stale = true
    }
  }, [activeRuntimeEnvironmentId, repoId, scope, attempt, lookupDefault])

  useEffect(() => {
    if (!searchable) {
      return
    }
    // Why: a response for an older query or repo must never replace the current one; the IPC cannot be aborted.
    let stale = false
    const timer = window.setTimeout(() => {
      searchRuntimeRepoBaseRefs(
        { activeRuntimeEnvironmentId },
        repoId,
        trimmedQuery,
        REPO_REF_SEARCH_RESULT_LIMIT
      )
        .then((results) => {
          if (!stale) {
            setOutcome({
              key: searchKey,
              status: results.length > 0 ? 'results' : 'empty',
              results
            })
          }
        })
        .catch((err: unknown) => {
          console.error('[RepoRefPicker] searchBaseRefs failed', err)
          if (!stale) {
            setOutcome({
              key: searchKey,
              status: isRepoRefHostOffline(err) ? 'offline' : 'error',
              results: []
            })
          }
        })
    }, REPO_REF_SEARCH_DEBOUNCE_MS)
    return () => {
      stale = true
      window.clearTimeout(timer)
    }
  }, [activeRuntimeEnvironmentId, repoId, searchKey, searchable, trimmedQuery])

  const setQuery = useCallback((text: string) => setQueryState({ scope, text }), [scope])
  const retry = useCallback(() => setAttempt((n) => n + 1), [])

  const scopedDefaults = defaults?.scope === scope ? defaults : null
  let status: RepoRefSearchStatus = 'idle'
  let results: readonly string[] = []
  if (searchEnabled && !withinLimit && trimmedQuery.length >= REPO_REF_SEARCH_MIN_QUERY_LENGTH) {
    // A query past the byte limit is never sent; it matches nothing.
    status = 'empty'
  } else if (searchable) {
    if (outcome?.key === searchKey) {
      status = outcome.status
      results = outcome.results
    } else {
      status = 'searching'
    }
  }

  return {
    query,
    setQuery,
    results,
    status,
    defaultBaseRef: scopedDefaults?.defaultBaseRef ?? null,
    remoteCount: scopedDefaults?.remoteCount ?? 0,
    defaultStatus: scopedDefaults?.status ?? 'loading',
    retry
  }
}
