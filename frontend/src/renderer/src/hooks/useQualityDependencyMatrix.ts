/**
 * useQualityDependencyMatrix.ts — FE-CV-TASK-087-15
 *
 * Module graph for the dependency matrix from `codeIntel.structure` (depth 2, the same request
 * as the Structure lens root). `architecture` is C4 and unrelated (PQ-10).
 *
 * @module hooks/useQualityDependencyMatrix
 */

import { useMemo } from 'react'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { resolveQualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { QualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { ModuleGraph } from '../../../shared/code-intel-graph-types'
import { defaultCodeIntelCall, useCodeIntelQuery } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError } from './useCodeIntelQuery'
import { useQualitySupport } from './useQualitySupport'
import type { QualitySupport } from './useQualitySupport'

const STRUCTURE_PARAMS = { depth: 2 }

export type UseQualityDependencyMatrixResult = {
  support: QualitySupport
  status: QualityBlockStatus
  graph: Partial<ModuleGraph> | null
  truncated: boolean
  totalCount: number
  error: CodeIntelQueryError | null
  refetch: () => void
}

function parseModuleGraph(raw: unknown): Partial<ModuleGraph> {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Partial<ModuleGraph>
  return {
    nodes: Array.isArray(r.nodes) ? r.nodes : [],
    edges: Array.isArray(r.edges) ? r.edges : []
  }
}

export function useQualityDependencyMatrix(
  worktreeId: string | null | undefined,
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): UseQualityDependencyMatrixResult {
  const support = useQualitySupport(worktreeId)
  const selector = useCodeIntelSelector(worktreeId ?? '')
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const environmentId = selector.state === 'ready' ? selector.environmentId : null
  const query = useCodeIntelQuery<Partial<ModuleGraph>>(
    worktreeId ?? null,
    environmentId,
    { method: 'structure', params: STRUCTURE_PARAMS, enabled, parseResult: parseModuleGraph },
    callFn
  )
  return useMemo(() => {
    let status = resolveQualityBlockStatus({
      enabled,
      hasData: query.data !== null,
      isEmpty: query.data !== null && (query.data.edges?.length ?? 0) === 0,
      loading: query.status === 'loading',
      failed: query.status === 'error',
      stale: query.stale
    })
    if (enabled && selector.state !== 'ready') {
      status = 'unavailable'
    }
    return {
      support,
      status,
      graph: query.data,
      truncated: query.truncated,
      totalCount: query.meta?.totalCount ?? 0,
      error: query.error,
      refetch: query.refetch
    }
  }, [
    support,
    enabled,
    selector.state,
    query.data,
    query.status,
    query.stale,
    query.meta,
    query.truncated,
    query.error,
    query.refetch
  ])
}
