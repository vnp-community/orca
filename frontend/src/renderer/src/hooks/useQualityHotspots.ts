/**
 * useQualityHotspots.ts — FE-CV-TASK-087-15
 *
 * Hotspot files = structural findings of rule `hotspot.file` (the contract has no hotspot
 * channel of its own). Goes through useCodeIntelQuery so the cache is pruned with the worktree.
 *
 * @module hooks/useQualityHotspots
 */

import { useMemo } from 'react'
import { useCodeIntelSelector } from '@/lib/code-intel-worktree-selector'
import { HOTSPOT_RULE } from '../components/review-map/quality/quality-hotspot-rows'
import { resolveQualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { QualityBlockStatus } from '../components/review-map/quality/quality-block-status-model'
import type { Finding } from '../../../shared/code-intel-findings-types'
import { parseFindingsPage } from './useCodeIntelFindings'
import { defaultCodeIntelCall, useCodeIntelQuery } from './useCodeIntelQuery'
import type { CodeIntelCallFn, CodeIntelQueryError } from './useCodeIntelQuery'
import { useQualitySupport } from './useQualitySupport'
import type { QualitySupport } from './useQualitySupport'

const HOTSPOT_PARAMS = { rules: [HOTSPOT_RULE], scope: 'all', limit: 100 } as const
const NO_FINDINGS: readonly Finding[] = []

export type UseQualityHotspotsResult = {
  support: QualitySupport
  status: QualityBlockStatus
  findings: readonly Finding[]
  totalCount: number
  truncated: boolean
  error: CodeIntelQueryError | null
  refetch: () => void
}

type Page = ReturnType<typeof parseFindingsPage>

export function useQualityHotspots(
  worktreeId: string | null | undefined,
  callFn: CodeIntelCallFn = defaultCodeIntelCall
): UseQualityHotspotsResult {
  const support = useQualitySupport(worktreeId)
  const selector = useCodeIntelSelector(worktreeId ?? '')
  const enabled = support === 'enabled' && Boolean(worktreeId)
  const environmentId = selector.state === 'ready' ? selector.environmentId : null
  const query = useCodeIntelQuery<Page>(
    worktreeId ?? null,
    environmentId,
    {
      method: 'findings',
      params: HOTSPOT_PARAMS as unknown as Record<string, unknown>,
      enabled,
      parseResult: parseFindingsPage
    },
    callFn
  )
  return useMemo(() => {
    const findings = query.data?.findings ?? NO_FINDINGS
    let status = resolveQualityBlockStatus({
      enabled,
      hasData: query.data !== null,
      isEmpty: query.data !== null && findings.length === 0,
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
      findings,
      totalCount: Math.max(query.meta?.totalCount ?? 0, findings.length),
      truncated: query.truncated,
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
