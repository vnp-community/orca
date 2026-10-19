/**
 * useSolutionPanelSync — CR-REQ-020-02
 *
 * Keeps the Analysis panel current without relying on events alone: re-reads Solutions and Approvals when the Request
 * moves on, and polls while a version is being written or its approval row has not arrived.
 *
 * @module components/request/solution/useSolutionPanelSync
 */

import { useEffect, useRef } from 'react'
import { useRequestCooldown } from '../../../hooks/useRequestCooldown'
import type {
  AnalysisRun,
  Approval,
  OrcaRequest,
  Solution
} from '../../../../../shared/request-types'

// Why: a failed run raises no event, so a request that is still analysing must be re-read.
const RUN_POLL_INTERVAL_MS = 5000
// Why: the approval row is written after the Solution; a missed event must not leave the bar missing until reload.
const APPROVAL_POLL_INTERVAL_MS = 3000
const APPROVAL_POLL_MAX_TRIES = 20
// Why: after an answered Clarification the new Solution is written by the backend, with no guaranteed event.
const FRESH_SOLUTION_POLL_INTERVAL_MS = 3000
const FRESH_SOLUTION_POLL_MAX_TRIES = 40

export type SolutionPanelSyncParams = {
  request: Pick<OrcaRequest, 'status' | 'updatedAt' | 'version'>
  /** The version on screen. */
  solution: Solution | undefined
  /** The newest version that has not been replaced. */
  latest: Solution | undefined
  approval: Approval | null
  expiredApproval: boolean
  latestRun: AnalysisRun | null
  refetchSolutions: () => void
  refetchApprovals: () => void
  onChanged: () => void
}

export function useSolutionPanelSync({
  request,
  solution,
  latest,
  approval,
  expiredApproval,
  latestRun,
  refetchSolutions,
  refetchApprovals,
  onChanged
}: SolutionPanelSyncParams): {
  failedRun: AnalysisRun | null
  runPending: boolean
  awaitingNewer: boolean
} {
  const readCooldown = useRequestCooldown('read')

  // Why: a Request that moved on (Clarification answered, regenerated) has a newer Solution; events can be missed,
  // so the Request's own state is the trigger. Approvals are not part of either payload, so they are re-read too.
  const requestKey = `${request.status}|${request.updatedAt ?? ''}|${request.version ?? ''}`
  const syncKey = `${requestKey}|${solution?.id ?? ''}|${solution?.status ?? ''}`
  const lastSyncKey = useRef(syncKey)
  const lastRequestKey = useRef(requestKey)
  useEffect(() => {
    if (lastSyncKey.current === syncKey) {
      return
    }
    lastSyncKey.current = syncKey
    if (lastRequestKey.current !== requestKey) {
      lastRequestKey.current = requestKey
      refetchSolutions()
    }
    refetchApprovals()
  }, [syncKey, requestKey, refetchApprovals, refetchSolutions])

  const awaitingApprovalRow =
    request.status === 'awaiting_analysis_approval' &&
    !!solution &&
    solution.status === 'ready' &&
    solution.id === latest?.id &&
    !approval &&
    !expiredApproval &&
    !readCooldown.coolingDown
  useEffect(() => {
    if (!awaitingApprovalRow) {
      return undefined
    }
    let tries = 0
    const t = setInterval(() => {
      tries += 1
      if (tries > APPROVAL_POLL_MAX_TRIES) {
        clearInterval(t)
        return
      }
      refetchApprovals()
    }, APPROVAL_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [awaitingApprovalRow, solution?.id, refetchApprovals])

  const failedRun = latestRun?.status === 'failed' ? latestRun : null
  const runPending = latestRun?.status === 'running'
  const polling =
    (runPending || (request.status === 'analyzing' && !latest && !failedRun)) &&
    !readCooldown.coolingDown
  useEffect(() => {
    if (!polling) {
      return undefined
    }
    const t = setInterval(refetchSolutions, RUN_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [polling, refetchSolutions])

  // Only superseded/rejected versions are on screen while the backend is producing the next one.
  const awaitingNewer =
    !!latest &&
    !runPending &&
    !failedRun &&
    ((latest.status === 'superseded' &&
      (request.status === 'analyzing' || request.status === 'awaiting_analysis_approval')) ||
      (latest.status === 'rejected' && request.status === 'analyzing'))
  useEffect(() => {
    if (!awaitingNewer || readCooldown.coolingDown) {
      return undefined
    }
    let tries = 0
    const t = setInterval(() => {
      tries += 1
      if (tries > FRESH_SOLUTION_POLL_MAX_TRIES) {
        clearInterval(t)
        return
      }
      refetchSolutions()
      refetchApprovals()
      // Why: the Request itself stays `analyzing` on screen until it is re-read.
      if (request.status === 'analyzing') {
        onChanged()
      }
    }, FRESH_SOLUTION_POLL_INTERVAL_MS)
    return () => clearInterval(t)
  }, [
    awaitingNewer,
    readCooldown.coolingDown,
    request.status,
    latest?.id,
    refetchSolutions,
    refetchApprovals,
    onChanged
  ])

  return { failedRun, runPending, awaitingNewer }
}
