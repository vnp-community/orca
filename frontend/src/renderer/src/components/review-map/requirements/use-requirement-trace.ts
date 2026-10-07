/**
 * use-requirement-trace.ts — FE-CV-TASK-092-02
 *
 * Hook for fetching and managing requirement trace data.
 * Read-only; uses generic useCodeIntelQuery.
 *
 * Error handling contract (spec 2.3):
 * - quality-disabled / disabled → state: 'disabled' (silent)
 * - Other errors → surfaces to caller via CodeIntelQueryResult.error
 *
 * @module components/review-map/requirements/use-requirement-trace
 */

import { useCodeIntelQuery } from '../../../hooks/useCodeIntelQuery'
import { useQualityFeatureFlags } from '../../../hooks/useQualityFeatureFlags'
import {
  buildRequirementTraceViewModel,
} from './requirement-trace-view-model'
import type { RequirementTrace, RequirementTraceViewModel } from './requirement-trace-view-model'
import type { CodeIntelQueryResult } from '../../../hooks/useCodeIntelQuery'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type RequirementTraceState = CodeIntelQueryResult<RequirementTraceViewModel> & {
  /** true when quality feature disabled — callers should hide the lens */
  disabled: boolean
  available: boolean
}

export type RequirementTraceOpts = {
  showInferred?: boolean
  projectId?: string | null
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useRequirementTrace(
  worktreeId: string | null,
  environmentId: string | null,
  opts: RequirementTraceOpts = {}
): RequirementTraceState {
  const flags = useQualityFeatureFlags(worktreeId)
  const { showInferred = false, projectId } = opts

  const enabled = flags.quality && Boolean(worktreeId) && Boolean(projectId)

  const queryResult = useCodeIntelQuery<RequirementTraceViewModel>(
    worktreeId,
    environmentId,
    {
      enabled,
      method: 'quality.trace',
      params: {
        worktreeId: worktreeId ?? '',
        projectId: projectId ?? '',
        includeInferred: showInferred,
      },
      parseResult: (raw) => {
        // raw is expected to be { traces: RequirementTrace[] }
        const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
        const traces = Array.isArray(r.traces) ? (r.traces as RequirementTrace[]) : []
        return buildRequirementTraceViewModel(traces)
      },
    }
  )

  // Map quality-disabled/disabled errors to 'disabled' state (silent)
  const isDisabledError =
    queryResult.error?.kind === 'quality-disabled' ||
    queryResult.error?.kind === 'disabled'

  return {
    ...queryResult,
    disabled: isDisabledError,
    available: enabled,
  }
}
