/**
 * useCodeIntelFindings.ts — FE-CV-TASK-059-03
 *
 * Structural `Finding` list (channel `codeIntel.findings`; QualityFinding is a separate source).
 * Page 1 goes through useCodeIntelQuery (cache, retry, stale push); later pages are appended and
 * deduplicated by findingKey. Optimistic dismissal state lives in an override map that is dropped
 * whenever the server data is reloaded, so the list never drifts from the backend.
 *
 * @module hooks/useCodeIntelFindings
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Finding, IndexFreshness } from '../../../shared/code-intel-types'
import { toCodeIntelMethod } from '../../../shared/code-intel-rpc-methods'
import {
  buildCodeIntelCacheKey,
  defaultCodeIntelCall,
  useCodeIntelQuery
} from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError, CodeIntelQueryStatus } from './useCodeIntelQuery'

export const FINDINGS_PAGE_LIMIT = 100

/** Filters the backend applies; kind / origin / search are client-side (finding-filter.ts). */
export type FindingsServerFilters = {
  severities?: readonly string[]
  pathPrefix?: string
  includeDismissed?: boolean
  scope?: 'all' | 'changed'
  base?: string
}

export type FindingDismissalOverride = Finding['dismissed'] | null

type FindingsPage = {
  findings: Finding[]
  dismissedCount: number
  indexFreshness: IndexFreshness | null
}

export function parseFindingsPage(raw: unknown): FindingsPage {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  return {
    findings: Array.isArray(r.findings) ? (r.findings as Finding[]) : [],
    dismissedCount: typeof r.dismissedCount === 'number' ? r.dismissedCount : 0,
    indexFreshness:
      typeof r.indexFreshness === 'object' && r.indexFreshness !== null
        ? (r.indexFreshness as IndexFreshness)
        : null
  }
}

function buildParams(filters: FindingsServerFilters): Record<string, unknown> {
  return {
    limit: FINDINGS_PAGE_LIMIT,
    scope: filters.scope ?? 'changed',
    ...(filters.severities?.length ? { severities: [...filters.severities] } : {}),
    ...(filters.pathPrefix ? { pathPrefix: filters.pathPrefix } : {}),
    ...(filters.includeDismissed ? { includeDismissed: true } : {}),
    ...(filters.base ? { base: filters.base } : {})
  }
}

export function mergeFindingPages(pages: readonly (readonly Finding[])[]): Finding[] {
  const seen = new Set<string>()
  const out: Finding[] = []
  for (const page of pages) {
    for (const finding of page) {
      if (!seen.has(finding.findingKey)) {
        seen.add(finding.findingKey)
        out.push(finding)
      }
    }
  }
  return out
}

export type UseCodeIntelFindingsResult = {
  status: CodeIntelQueryStatus
  error: CodeIntelQueryError | null
  findings: Finding[]
  dismissedCount: number
  indexFreshness: IndexFreshness | null
  stale: boolean
  hasNextPage: boolean
  isFetchingNextPage: boolean
  nextPageError: CodeIntelQueryError | null
  loadMore: () => void
  reload: () => void
  setOverride: (findingKey: string, dismissed: FindingDismissalOverride) => void
  clearOverride: (findingKey: string) => void
}

export function useCodeIntelFindings(
  worktreeId: string | null,
  environmentId: string | null,
  filters: FindingsServerFilters,
  opts: { enabled?: boolean } = {},
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): UseCodeIntelFindingsResult {
  const severitiesKey = filters.severities?.join(',') ?? ''
  const { scope, pathPrefix, includeDismissed, base } = filters
  const params = useMemo(
    () =>
      buildParams({
        scope,
        pathPrefix,
        includeDismissed,
        base,
        severities: severitiesKey ? severitiesKey.split(',') : undefined
      }),
    [scope, pathPrefix, includeDismissed, base, severitiesKey]
  )
  const first = useCodeIntelQuery<FindingsPage>(
    worktreeId,
    environmentId,
    { method: 'findings', params, enabled: opts.enabled, parseResult: parseFindingsPage },
    callFn
  )
  const queryKey = buildCodeIntelCacheKey('findings', params)

  const [extra, setExtra] = useState<{ key: string; pages: Finding[][]; token: string | null } | null>(null)
  const [isFetchingNextPage, setFetching] = useState(false)
  const [nextPageError, setNextPageError] = useState<CodeIntelQueryError | null>(null)
  const [overrides, setOverrides] = useState<Record<string, FindingDismissalOverride>>({})
  const inFlightRef = useRef(false)
  const generationRef = useRef(0)

  // New params or a reloaded first page invalidate appended pages, overrides and any page in flight.
  const firstData = first.data
  useEffect(() => {
    generationRef.current++
    inFlightRef.current = false
    setExtra(null)
    setFetching(false)
    setNextPageError(null)
    setOverrides({})
  }, [queryKey, firstData])

  const appended = extra && extra.key === queryKey ? extra : null
  const token = appended ? appended.token : (first.meta?.nextPageToken ?? null)

  const loadMore = useCallback(() => {
    if (!worktreeId || !token || inFlightRef.current || first.status !== 'success') {
      return
    }
    inFlightRef.current = true
    setFetching(true)
    setNextPageError(null)
    const generation = generationRef.current
    const ctrl = new AbortController()
    void callFn(worktreeId, toCodeIntelMethod('findings'), { ...params, pageToken: token }, ctrl.signal, environmentId)
      .then((outcome) => {
        if (generation !== generationRef.current) {
          return
        }
        if (!outcome.ok) {
          setNextPageError(outcome.error)
          return
        }
        const page = parseFindingsPage(outcome.result)
        setExtra((prev) => ({
          key: queryKey,
          pages: [...(prev && prev.key === queryKey ? prev.pages : []), page.findings],
          token: outcome.meta?.nextPageToken ?? null
        }))
      })
      .catch(() => {
        if (generation === generationRef.current) {
          setNextPageError({ kind: 'unknown', code: null, message: 'Unexpected error', retryable: false })
        }
      })
      .finally(() => {
        if (generation === generationRef.current) {
          inFlightRef.current = false
          setFetching(false)
        }
      })
  }, [worktreeId, token, first.status, callFn, params, environmentId, queryKey])

  const setOverride = useCallback((findingKey: string, dismissed: FindingDismissalOverride) => {
    setOverrides((prev) => ({ ...prev, [findingKey]: dismissed }))
  }, [])
  const clearOverride = useCallback((findingKey: string) => {
    setOverrides((prev) => {
      if (!(findingKey in prev)) {
        return prev
      }
      const next = { ...prev }
      delete next[findingKey]
      return next
    })
  }, [])

  const serverFindings = useMemo(
    () => mergeFindingPages([firstData?.findings ?? [], ...(appended?.pages ?? [])]),
    [firstData, appended]
  )
  const { findings, dismissedCount } = useMemo(() => {
    let delta = 0
    const merged = serverFindings.map((finding) => {
      if (!(finding.findingKey in overrides)) {
        return finding
      }
      const override = overrides[finding.findingKey]
      if (Boolean(finding.dismissed) !== Boolean(override)) {
        delta += override ? 1 : -1
      }
      return { ...finding, dismissed: override ?? undefined }
    })
    return { findings: merged, dismissedCount: Math.max(0, (firstData?.dismissedCount ?? 0) + delta) }
  }, [serverFindings, overrides, firstData])

  return {
    status: first.status,
    error: first.error,
    findings,
    dismissedCount,
    indexFreshness: firstData?.indexFreshness ?? null,
    stale: first.stale,
    hasNextPage: token !== null,
    isFetchingNextPage,
    nextPageError,
    loadMore,
    reload: first.refetch,
    setOverride,
    clearOverride
  }
}
