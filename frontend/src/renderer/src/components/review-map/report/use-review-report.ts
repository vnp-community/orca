/**
 * use-review-report.ts — FE-CV-TASK-090-05
 *
 * On-demand `quality.report` fetch. Nothing is requested until the user acts
 * (the call can take up to 20 s), and nothing at all while the quality flag is off.
 *
 * @module components/review-map/report/use-review-report
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/store'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { getCodeIntelClient } from '../../../runtime/code-intel-client'
import { CODE_INTEL_RPC_METHODS } from '../../../../../shared/code-intel-rpc-methods'
import { parseReviewReportModel } from './review-report-model-parser'
import type { ReviewReportModel } from './review-report-model-parser'

const MAX_RETRY_MS = 90_000
const DEFAULT_RETRY_MS = 3000
// Kinds that mean "no report here"; the UI hides silently instead of showing an error.
const SILENT_KINDS = new Set(['disabled', 'unsupported', 'forbidden', 'quality-disabled', 'no-binding', 'not-found'])

export type ReviewReportFetchResult =
  | { ok: true; model: ReviewReportModel }
  | { ok: false; reason: 'unavailable' | 'error' | 'aborted' }

export type UseReviewReportResult = {
  /** False when the quality flag is off or the worktree has no projectId. */
  available: boolean
  loading: boolean
  fetchReport: () => Promise<ReviewReportFetchResult>
}

function retryDelay(data: Record<string, unknown> | null): number {
  const value = data?.retryAfterMs
  return typeof value === 'number' && value > 0 && value <= 30_000 ? value : DEFAULT_RETRY_MS
}

export function useReviewReport(opts: {
  worktreeId: string | null
  projectId: string | null | undefined
  base?: string | null
}): UseReviewReportResult {
  const { worktreeId, projectId, base } = opts
  const { quality } = useQualityFeatureFlags()
  const available = quality && Boolean(worktreeId) && Boolean(projectId)
  const [loading, setLoading] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const inFlightRef = useRef(false)

  useEffect(() => () => abortRef.current?.abort(), [])

  const fetchReport = useCallback(async (): Promise<ReviewReportFetchResult> => {
    if (!available || !worktreeId) {
      return { ok: false, reason: 'unavailable' }
    }
    // Why: lock immediately so a double click cannot start a second 20 s request.
    if (inFlightRef.current) {
      return { ok: false, reason: 'aborted' }
    }
    inFlightRef.current = true
    setLoading(true)
    const ctrl = new AbortController()
    abortRef.current = ctrl
    const startedAt = Date.now()
    try {
      const client = getCodeIntelClient()
      for (;;) {
        const response = await client.call(
          worktreeId,
          CODE_INTEL_RPC_METHODS.QUALITY_REPORT,
          { projectId, worktreeId, ...(base ? { base } : {}) },
          {
            environmentId: getRuntimeEnvironmentIdForWorktree(useAppStore.getState(), worktreeId),
            signal: ctrl.signal
          }
        )
        if (ctrl.signal.aborted) {
          return { ok: false, reason: 'aborted' }
        }
        if (response.ok) {
          const result = response.result as { model?: unknown } | null
          return { ok: true, model: parseReviewReportModel(result?.model ?? result) }
        }
        if (SILENT_KINDS.has(response.error.kind)) {
          return { ok: false, reason: 'unavailable' }
        }
        if (response.error.message?.includes('inProgress') && Date.now() - startedAt < MAX_RETRY_MS) {
          await new Promise((resolve) => setTimeout(resolve, retryDelay(response.error.data)))
          continue
        }
        return { ok: false, reason: 'error' }
      }
    } catch {
      return { ok: false, reason: 'error' }
    } finally {
      inFlightRef.current = false
      setLoading(false)
    }
  }, [available, worktreeId, projectId, base])

  return { available, loading, fetchReport }
}
