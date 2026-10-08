/**
 * use-review-surface-telemetry.ts — FE-CV-TASK-095-06
 *
 * `review_opened` once per workspace open (when data first renders) and `review_lens_viewed` for
 * a lens kept in view >= 2 s, at most once per lens per open. Only enums and buckets leave.
 *
 * @module components/review-map/shell/use-review-surface-telemetry
 */

import { useEffect, useRef } from 'react'
import { takeReviewOpenSource } from '@/lib/review-open-source'
import { trackReviewLensViewed, trackReviewOpened } from '@/lib/review-telemetry'
import type { IndexStatusView } from '../review-wire-types'
import type { ReviewScope } from '../review-scope-model'

const LENS_MIN_DWELL_MS = 2000
const TELEMETRY_LENSES = new Set([
  'impact',
  'architecture',
  'dataflow',
  'erd',
  'storage',
  'structure',
  'contract',
  'quality',
  'requirements'
])

type TelemetryLens = Parameters<typeof trackReviewLensViewed>[0]['lens']

export function telemetryScopeKind(
  scope: ReviewScope,
  defaultBaseRef: string | null
): 'merge_base' | 'committed' | 'custom_base' {
  if (scope.kind === 'range') {
    return 'committed'
  }
  if (scope.kind === 'branch') {
    return defaultBaseRef === null || scope.baseRef === defaultBaseRef ? 'merge_base' : 'custom_base'
  }
  return 'custom_base'
}

export function telemetryIndexState(
  overall: IndexStatusView['overall'] | undefined
): 'fresh' | 'stale' | 'missing' | 'unknown' {
  switch (overall) {
    case 'READY':
      return 'fresh'
    case 'STALE':
    case 'OVERLAY':
    case 'DEGRADED':
      return 'stale'
    case 'MISSING':
    case 'NOT_INSTALLED':
      return 'missing'
    default:
      return 'unknown'
  }
}

export function useReviewSurfaceTelemetry(args: {
  worktreeId: string
  /** True once the overlay is on screen (not a loading/error screen). */
  ready: boolean
  scope: ReviewScope | null
  defaultBaseRef: string | null
  overall: IndexStatusView['overall'] | undefined
  activeLensId: string | null
}): void {
  const { worktreeId, ready, scope, defaultBaseRef, overall, activeLensId } = args
  const openedRef = useRef(false)
  const latest = useRef({ scope, defaultBaseRef, overall, activeLensId })
  latest.current = { scope, defaultBaseRef, overall, activeLensId }

  useEffect(() => {
    openedRef.current = false
  }, [worktreeId])

  useEffect(() => {
    const cur = latest.current
    if (!ready || openedRef.current || !cur.scope) {
      return
    }
    openedRef.current = true
    const origin = takeReviewOpenSource(worktreeId)
    trackReviewOpened({
      source: origin.source,
      scope: telemetryScopeKind(cur.scope, cur.defaultBaseRef),
      lens: (cur.activeLensId && TELEMETRY_LENSES.has(cur.activeLensId)
        ? cur.activeLensId
        : 'impact') as TelemetryLens,
      index: telemetryIndexState(cur.overall),
      after_agent_turn: origin.afterAgentTurn
    })
  }, [ready, worktreeId])

  const reported = useRef(new Set<string>())
  useEffect(() => {
    reported.current = new Set()
  }, [worktreeId])
  useEffect(() => {
    if (!ready || !activeLensId || !TELEMETRY_LENSES.has(activeLensId)) {
      return
    }
    const startedAt = Date.now()
    const lens = activeLensId
    // Report on leaving the lens (or unmount) so the dwell bucket is the real time spent.
    return () => {
      const dwellMs = Date.now() - startedAt
      if (dwellMs >= LENS_MIN_DWELL_MS && !reported.current.has(lens)) {
        reported.current.add(lens)
        trackReviewLensViewed({ lens: lens as TelemetryLens, dwellMs })
      }
    }
  }, [ready, activeLensId])
}
