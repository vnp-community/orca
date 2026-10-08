/**
 * useTaskReadiness — FE-REQ-TASK-036-02
 *
 * Task-level readiness (CR-REQ-029). An unsupported runtime never locks Run.
 *
 * @module hooks/useTaskReadiness
 */

import { useCallback, useEffect, useMemo, useState } from 'react'
import { callRequestRpc, type Result } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseTaskReadinessReport } from '../../../shared/request-artifact-parsers'
import type { TaskReadinessReport } from '../../../shared/request-artifact-types'

export type TaskReadinessStatus = 'idle' | 'loading' | 'ready' | 'unsupported'
export type PhaseReadinessSummary = { ready: number; needsInfo: number; specDefect: number; envDefect: number; unchecked: number }

export function summarizePhaseReadiness(taskIds: readonly string[], byTaskId: Record<string, TaskReadinessReport>): PhaseReadinessSummary {
  const out: PhaseReadinessSummary = { ready: 0, needsInfo: 0, specDefect: 0, envDefect: 0, unchecked: 0 }
  for (const id of taskIds) {
    switch (byTaskId[id]?.outcome) {
      case 'ready': out.ready += 1; break
      case 'needs_info': out.needsInfo += 1; break
      case 'spec_defect': out.specDefect += 1; break
      case 'env_defect': out.envDefect += 1; break
      default: out.unchecked += 1
    }
  }
  return out
}

type Params = { requestId?: string | null; taskId?: string; phaseId?: string; phaseTaskIds?: readonly string[] }

export function useTaskReadiness({ requestId = null, taskId, phaseId, phaseTaskIds = [] }: Params) {
  const [byTaskId, setByTaskId] = useState<Record<string, TaskReadinessReport>>({})
  const [status, setStatus] = useState<TaskReadinessStatus>('idle')
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick((n) => n + 1), [])
  useRefetchOnRequestEvent(requestId, ['readiness.reported'], refetch)

  useEffect(() => {
    if (!taskId && !phaseId) {return}
    let cancelled = false
    setStatus('loading')
    const call = phaseId
      ? callRequestRpc<{ reports?: unknown[] }>(REQUEST_RPC_METHODS.READINESS_LIST, { phaseId })
      : callRequestRpc<{ report?: unknown }>(REQUEST_RPC_METHODS.READINESS_GET, { taskId })
    void call.then((res) => {
      if (cancelled) {return}
      if (!res.ok) {
        // Why: no readiness channel keeps the SOL-021 behavior (Run is not locked).
        setStatus(res.error.kind === 'unsupported' ? 'unsupported' : 'idle')
        return
      }
      const raw = res.value as { reports?: unknown[]; report?: unknown }
      const list = phaseId ? raw.reports ?? [] : [raw.report ?? res.value]
      const next: Record<string, TaskReadinessReport> = {}
      for (const item of list) {
        const rep = parseTaskReadinessReport(item)
        if (rep) {next[rep.taskId] = rep}
      }
      setByTaskId((prev) => ({ ...prev, ...next }))
      setStatus('ready')
    })
    return () => { cancelled = true }
  }, [taskId, phaseId, tick])

  const check = useCallback(async (id: string): Promise<Result<TaskReadinessReport | null>> => {
    const res = await callRequestRpc<{ report?: unknown }>(REQUEST_RPC_METHODS.READINESS_CHECK, { taskId: id })
    if (!res.ok) {return res}
    const rep = parseTaskReadinessReport(res.value.report ?? res.value)
    if (rep) {setByTaskId((prev) => ({ ...prev, [rep.taskId]: rep }))}
    return { ok: true, value: rep }
  }, [])

  const phaseSummary = useMemo(() => summarizePhaseReadiness(phaseTaskIds, byTaskId), [phaseTaskIds, byTaskId])
  return { report: taskId ? byTaskId[taskId] ?? null : null, byTaskId, phaseSummary, status, check, refetch }
}
