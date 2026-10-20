/**
 * usePlanProposalOfRun — CR-REQ-045
 *
 * `request.planProposal` for one Plan version: the native proposal, or the OpenSpec change's file index with its checks. Asked
 * for a version by its run (and revision number when the gateway lists them), not for "the latest run", which is empty while a
 * change request is being written and after one failed. Without either it reads the latest run, as a gateway older than
 * CR-REQ-045 can only do.
 *
 * It reads once per (run, revision, refreshKey); following a run that is still writing is usePlanRunState's job.
 *
 * @module hooks/usePlanProposalOfRun
 */

import { useCallback, useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parsePlanProposalView } from '../../../shared/request-plan-revision-parsers'
import type { PlanProposalView } from '../../../shared/request-plan-revision'

export type PlanProposalOfRun = {
  view: PlanProposalView | null
  isLoading: boolean
  /** RequestRpcErrorKind of the failed read; null otherwise. */
  error: string | null
  /** The run is still writing: there is no proposal to show yet. */
  running: boolean
  refetch: () => void
}

type Fetched = { key: string; view: PlanProposalView | null; error: string | null }

export function usePlanProposalOfRun(
  requestId: string,
  which: { runId?: string; revision?: number; refreshKey?: string },
  enabled = true
): PlanProposalOfRun {
  const runId = which.runId ?? ''
  const revision = which.revision && which.revision > 0 ? which.revision : 0
  const key = `${requestId}|${runId}|${revision}|${which.refreshKey ?? ''}`
  const [fetched, setFetched] = useState<Fetched | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    if (!enabled || !requestId) {
      return
    }
    let cancelled = false
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.PLAN_PROPOSAL, {
      id: requestId,
      ...(runId ? { runId } : {}),
      ...(revision ? { revision } : {})
    }).then((res) => {
      if (cancelled) {
        return
      }
      setFetched(
        res.ok
          ? { key, view: parsePlanProposalView(res.value), error: null }
          : { key, view: null, error: res.error.kind }
      )
    })
    return () => {
      cancelled = true
    }
  }, [enabled, requestId, runId, revision, key, attempt])

  const refetch = useCallback(() => setAttempt((n) => n + 1), [])
  const mine = fetched?.key === key ? fetched : null
  // Why: a refresh of the same version keeps showing the last answer; another version never shows this one's.
  const stale =
    fetched && fetched.key.startsWith(`${requestId}|${runId}|${revision}|`) ? fetched : null
  const shown = mine ?? stale
  return {
    view: shown?.view ?? null,
    isLoading: enabled && mine === null,
    error: mine?.error ?? null,
    running: mine?.view?.status === 'running',
    refetch
  }
}
