/**
 * useDataFlow.ts — FE-CV-TASK-056-05
 *
 * One flow with its server-built SequenceModel. Aborts when the flow or detail changes.
 */

import { useMemo } from 'react'
import type { DataFlow, SequenceModel } from '../../../shared/code-intel-architecture-types'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { useCodeIntelViewLoad } from './useCodeIntelViewLoad'
import type { ViewLoadResult } from './useCodeIntelViewLoad'

export type DataFlowDetail = 'service' | 'component'
export type DataFlowData = { flow: DataFlow; sequence: SequenceModel | null }

export const DATA_FLOW_MAX_STEPS = 200

export function parseDataFlow(raw: unknown): DataFlowData {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  const f = r.flow as Record<string, unknown> | undefined
  if (!f || typeof f !== 'object') {
    throw new Error('dataFlow response has no flow')
  }
  const arr = <T,>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : [])
  const seq = r.sequence as Record<string, unknown> | undefined
  return {
    flow: {
      ...(f as unknown as DataFlow),
      steps: arr(f.steps),
      stores: arr(f.stores),
      gaps: arr(f.gaps),
      services: arr(f.services),
      relatedProcesses: arr(f.relatedProcesses)
    },
    sequence:
      seq && typeof seq === 'object'
        ? { participants: arr(seq.participants), messages: arr(seq.messages) }
        : null
  }
}

export function useDataFlow(
  worktreeId: string,
  environmentId: string | null,
  flowId: string | null,
  detail: DataFlowDetail
): ViewLoadResult<DataFlowData> {
  const params = useMemo(
    () =>
      flowId
        ? { flowId, detail, maxSteps: DATA_FLOW_MAX_STEPS, includeSequence: true, includeDfd: false }
        : null,
    [flowId, detail]
  )
  return useCodeIntelViewLoad<DataFlowData>({
    worktreeId,
    environmentId,
    method: CODE_INTEL_RPC_METHODS.DATA_FLOW,
    params,
    parse: parseDataFlow
  })
}
