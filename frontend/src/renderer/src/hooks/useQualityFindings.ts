/**
 * useQualityFindings.ts — FE-CV-TASK-087-12
 *
 * Paged list of `quality.findings` for the dock. Server filters: severities, categories,
 * inScope, file. Pages (limit 500) are joined by `nextPageToken`, de-duplicated by fingerprint
 * and capped at 5 000 rows; the cap is reported, never applied silently. Reloads when the
 * quality `epoch` changes (a run finished or the gate moved). No call unless support is enabled.
 *
 * @module hooks/useQualityFindings
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { getCodeIntelClient } from '../runtime/code-intel-client'
import type { CodeIntelRpcError } from '../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { parseQualityFindingsResponse } from '../../../shared/code-intel-quality-wire-parsers'
import type {
  QualityCategory,
  QualityFinding,
  QualityFindingWaiver,
  QualitySeverity
} from '../../../shared/code-intel-quality-types'
import { useQualitySupport } from './useQualitySupport'

export const QUALITY_FINDINGS_PAGE_LIMIT = 500
export const QUALITY_FINDINGS_MAX_ITEMS = 5000

export type QualityFindingsServerFilter = {
  severities: readonly QualitySeverity[]
  categories: readonly QualityCategory[]
  inScope: boolean
  file?: string
}

export type UseQualityFindingsResult = {
  status: 'idle' | 'loading' | 'ready' | 'error'
  items: QualityFinding[]
  total: number
  truncated: boolean
  /** More rows exist on the server than are loaded (page tokens left, or the 5 000 cap hit). */
  hasMore: boolean
  capped: boolean
  loadingMore: boolean
  outsideScopeCount: number
  error: CodeIntelRpcError | null
  loadMore: () => void
  retry: () => void
  /** Sets or clears the waiver of one row locally; returns a function that restores the row. */
  patchWaiver: (fingerprint: string, waiver: QualityFindingWaiver | undefined) => () => void
}

type Loaded = {
  items: QualityFinding[]
  total: number
  truncated: boolean
  outsideScopeCount: number
  nextPageToken?: string
}

export function mergeQualityFindingPages(
  current: readonly QualityFinding[],
  incoming: readonly QualityFinding[]
): { items: QualityFinding[]; capped: boolean } {
  const seen = new Set(current.map((f) => f.fingerprint))
  const items = [...current]
  let capped = false
  for (const finding of incoming) {
    if (seen.has(finding.fingerprint)) {
      continue
    }
    if (items.length >= QUALITY_FINDINGS_MAX_ITEMS) {
      capped = true
      break
    }
    seen.add(finding.fingerprint)
    items.push(finding)
  }
  return { items, capped }
}

function buildParams(
  filter: QualityFindingsServerFilter,
  pageToken?: string
): Record<string, unknown> {
  return {
    limit: QUALITY_FINDINGS_PAGE_LIMIT,
    ...(filter.severities.length > 0 ? { severities: filter.severities } : {}),
    ...(filter.categories.length > 0 ? { categories: filter.categories } : {}),
    ...(filter.inScope ? { inScope: true } : {}),
    ...(filter.file ? { file: filter.file } : {}),
    ...(pageToken ? { pageToken } : {})
  }
}

export function useQualityFindings(
  worktreeId: string | null | undefined,
  filter: QualityFindingsServerFilter
): UseQualityFindingsResult {
  const support = useQualitySupport(worktreeId)
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const epoch = useAppStore((s) =>
    worktreeId ? (s.codeIntelQualityByWorktree[worktreeId]?.epoch ?? 0) : 0
  )
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const [capped, setCapped] = useState(false)
  const [status, setStatus] = useState<UseQualityFindingsResult['status']>('idle')
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<CodeIntelRpcError | null>(null)
  const [retryNonce, setRetryNonce] = useState(0)
  const sequence = useRef(0)
  const loadedRef = useRef<Loaded | null>(null)
  loadedRef.current = loaded

  const { severities, categories, inScope, file } = filter
  const filterKey = `${severities.join(',')}|${categories.join(',')}|${inScope}|${file ?? ''}`

  const fetchPage = useCallback(
    async (pageToken: string | undefined, seq: number): Promise<Loaded | null> => {
      const outcome = await getCodeIntelClient()
        .call(
          worktreeId as string,
          CODE_INTEL_RPC_METHODS.QUALITY_FINDINGS,
          buildParams({ severities, categories, inScope, file }, pageToken),
          {}
        )
        .catch(() => null)
      if (seq !== sequence.current) {
        return null
      }
      if (!outcome || !outcome.ok) {
        setError(
          outcome && !outcome.ok
            ? outcome.error
            : {
                kind: 'unknown',
                code: null,
                message: 'Unexpected error',
                data: null,
                retryable: false
              }
        )
        setStatus('error')
        return null
      }
      const page = parseQualityFindingsResponse(outcome.result)
      const merged = mergeQualityFindingPages(
        pageToken ? (loadedRef.current?.items ?? []) : [],
        page.findings
      )
      setCapped(
        merged.capped ||
          (merged.items.length >= QUALITY_FINDINGS_MAX_ITEMS && Boolean(page.nextPageToken))
      )
      return {
        items: merged.items,
        total: page.totalCount,
        truncated: page.truncated,
        outsideScopeCount: page.outsideScopeCount,
        nextPageToken: page.nextPageToken
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [worktreeId, filterKey]
  )

  useEffect(() => {
    if (!enabled) {
      return
    }
    const seq = ++sequence.current
    setStatus('loading')
    setError(null)
    setCapped(false)
    void fetchPage(undefined, seq).then((next) => {
      if (next) {
        setLoaded(next)
        setStatus('ready')
      }
    })
    return () => {
      sequence.current += 1
    }
  }, [enabled, fetchPage, epoch, retryNonce])

  const loadMore = useCallback(() => {
    const current = loadedRef.current
    if (!enabled || !current?.nextPageToken || loadingMore || capped || status === 'loading') {
      return
    }
    const seq = sequence.current
    setLoadingMore(true)
    void fetchPage(current.nextPageToken, seq).then((next) => {
      setLoadingMore(false)
      if (next) {
        setLoaded(next)
      }
    })
  }, [enabled, loadingMore, capped, status, fetchPage])

  const retry = useCallback(() => setRetryNonce((n) => n + 1), [])

  const patchWaiver = useCallback(
    (fingerprint: string, waiver: QualityFindingWaiver | undefined): (() => void) => {
      const original = loadedRef.current?.items.find((f) => f.fingerprint === fingerprint)
      const replace = (next: QualityFinding | undefined): void =>
        setLoaded((cur) =>
          cur && next
            ? { ...cur, items: cur.items.map((f) => (f.fingerprint === fingerprint ? next : f)) }
            : cur
        )
      if (original) {
        const { waiver: _previous, ...rest } = original
        replace(waiver ? { ...rest, waiver } : rest)
      }
      return () => replace(original)
    },
    []
  )

  return useMemo(
    () => ({
      status: enabled ? status : 'idle',
      items: enabled && loaded ? loaded.items : EMPTY_ITEMS,
      total: loaded?.total ?? 0,
      truncated: loaded?.truncated ?? false,
      hasMore: Boolean(loaded?.nextPageToken) || capped,
      capped,
      loadingMore,
      outsideScopeCount: loaded?.outsideScopeCount ?? 0,
      error: enabled ? error : null,
      loadMore,
      retry,
      patchWaiver
    }),
    [enabled, status, loaded, capped, loadingMore, error, loadMore, retry, patchWaiver]
  )
}

const EMPTY_ITEMS: QualityFinding[] = []
