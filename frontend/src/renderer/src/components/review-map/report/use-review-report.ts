/**
 * use-review-report.ts — FE-CV-TASK-090-05
 *
 * Hook for fetching and managing the review report.
 * Uses the generic useCodeIntelQuery pattern.
 *
 * @module components/review-map/report/use-review-report
 */

import { useCallback } from 'react'
import { useCodeIntelQuery } from '../../../hooks/useCodeIntelQuery'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import { parseReviewReportModel } from './review-report-model-parser'
import type { ReviewReportModel } from './review-report-model-parser'
import type { CodeIntelQueryResult } from '../../../hooks/useCodeIntelQuery'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ReviewReportState = CodeIntelQueryResult<ReviewReportModel> & {
  /** Whether report fetching is available (quality flag enabled) */
  available: boolean
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useReviewReport(
  worktreeId: string | null,
  environmentId: string | null,
  params: { projectId?: string | null; baseRef?: string | null }
): ReviewReportState {
  const flags = useQualityFeatureFlags(worktreeId)

  const enabled = flags.quality && Boolean(worktreeId) && Boolean(params.projectId)

  const queryResult = useCodeIntelQuery<ReviewReportModel>(
    worktreeId,
    environmentId,
    {
      enabled,
      method: 'quality.report',
      params: {
        projectId: params.projectId ?? '',
        worktreeId: worktreeId ?? '',
        ...(params.baseRef ? { baseRef: params.baseRef } : {}),
      },
      parseResult: parseReviewReportModel,
    }
  )

  return {
    ...queryResult,
    available: enabled,
  }
}

// ---------------------------------------------------------------------------
// Export actions (standalone functions, not React hooks)
// ---------------------------------------------------------------------------

export { buildReviewReportExportActions } from './review-report-export-actions'
