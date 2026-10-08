/**
 * useApprovalInbox — CR-REQ-022-02
 *
 * Paged approval.listPending with client-side filters, 30 s polling, event
 * driven refetch and approve/reject that never throw. Separate from
 * useApprovals (per-request, unpaged) on purpose.
 *
 * @module hooks/useApprovalInbox
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseApproval } from '../../../shared/request-wire-parsers'
import { classifyApprovalDecisionError, type ApprovalDecisionOutcome } from '../lib/approval-decision-outcome'
import {
  isOverdue, matchesGroup, serverSubjectTypeFor, type ApprovalSubjectGroup
} from '../components/request/approval/approval-inbox-rules'
import { REJECT_REASON_MIN_LENGTH } from '../components/request/solution/solution-view-model'
import { useRequestSubscription } from './useRequestSubscription'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { Approval } from '../../../shared/request-types'

export const APPROVAL_INBOX_PAGE_SIZE = 50
export const APPROVAL_INBOX_POLL_MS = 30_000
export const APPROVAL_INBOX_EVENT_DEBOUNCE_MS = 500

export type ApprovalDecisionResult = { outcome: ApprovalDecisionOutcome; code?: string }

type Options = {
  subjectGroup: ApprovalSubjectGroup
  projectId?: string
  overdueOnly: boolean
  active: boolean
  /** Test seam; the screen passes the minute clock. */
  now?: number
}

type PendingPage = { approvals?: unknown[]; nextPageToken?: string }

export function useApprovalInbox({ subjectGroup, projectId, overdueOnly, active, now }: Options) {
  const requestsById = useAppStore((s) => s.requestsById)
  const supportState = useAppStore((s) => s.requestFlowSupport)
  const setPendingApprovalCount = useAppStore((s) => s.setPendingApprovalCount)
  const setRequestFlowSupport = useAppStore((s) => s.setRequestFlowSupport)

  const [rows, setRows] = useState<Approval[]>([])
  const [nextPageToken, setNextPageToken] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [error, setError] = useState<RequestRpcError | null>(null)
  const [refetchKey, setRefetchKey] = useState(0)

  const serverType = serverSubjectTypeFor(subjectGroup)
  // Why: ignore responses from superseded loads and from after unmount.
  const generation = useRef(0)
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  const refetch = useCallback(() => setRefetchKey((n) => n + 1), [])

  useEffect(() => {
    if (!active) {return}
    const gen = ++generation.current
    setIsLoading(true)
    void callRequestRpc<PendingPage>(REQUEST_RPC_METHODS.APPROVAL_LIST_PENDING, {
      pageSize: APPROVAL_INBOX_PAGE_SIZE,
      ...(serverType ? { subjectType: serverType } : {})
    }).then((result) => {
      if (!alive.current || gen !== generation.current) {return}
      setIsLoading(false)
      if (!result.ok) {
        if (result.error.kind === 'unsupported') {setRequestFlowSupport('unsupported')}
        // Keep the previous rows so the UI can dim them instead of blanking.
        setError(result.error)
        return
      }
      const parsed = (result.value.approvals ?? []).map(parseApproval)
      const token = result.value.nextPageToken || null
      setError(null)
      setRows(parsed)
      setNextPageToken(token)
      // Why: the sidebar dot must not depend on the filter in view or on a partial page.
      if (!serverType) {
        const current = useAppStore.getState().pendingApprovalCount
        setPendingApprovalCount(token ? Math.max(current, parsed.length) : parsed.length)
      }
    })
  }, [active, serverType, refetchKey, setPendingApprovalCount, setRequestFlowSupport])

  const loadMore = useCallback(() => {
    if (!nextPageToken || isLoadingMore) {return}
    const gen = generation.current
    setIsLoadingMore(true)
    void callRequestRpc<PendingPage>(REQUEST_RPC_METHODS.APPROVAL_LIST_PENDING, {
      pageSize: APPROVAL_INBOX_PAGE_SIZE,
      pageToken: nextPageToken,
      ...(serverType ? { subjectType: serverType } : {})
    }).then((result) => {
      if (!alive.current || gen !== generation.current) {return}
      setIsLoadingMore(false)
      if (!result.ok) {
        setError(result.error)
        return
      }
      const parsed = (result.value.approvals ?? []).map(parseApproval)
      setRows((prev) => {
        const seen = new Set(prev.map((r) => r.id))
        return [...prev, ...parsed.filter((r) => !seen.has(r.id))]
      })
      setNextPageToken(result.value.nextPageToken || null)
    })
  }, [nextPageToken, isLoadingMore, serverType])

  // Polling only while the tab is open and the window visible.
  useEffect(() => {
    if (!active) {return}
    const tick = (): void => {
      if (document.visibilityState === 'visible') {refetch()}
    }
    const timer = setInterval(tick, APPROVAL_INBOX_POLL_MS)
    document.addEventListener('visibilitychange', tick)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', tick)
    }
  }, [active, refetch])

  // Events carry no approval id, so they only trigger a debounced reload.
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (debounce.current) {clearTimeout(debounce.current)} }, [])
  useRequestSubscription({
    onEvent: (event) => {
      if (!active || !event.eventType.includes('approval.')) {return}
      if (debounce.current) {clearTimeout(debounce.current)}
      debounce.current = setTimeout(refetch, APPROVAL_INBOX_EVENT_DEBOUNCE_MS)
    }
  })

  const clock = now ?? Date.now()
  const filtered = useMemo(
    () =>
      rows.filter((row) => {
        if (!matchesGroup(row, subjectGroup)) {return false}
        if (overdueOnly && !isOverdue(row, clock)) {return false}
        // Rows whose Request summary has not loaded yet stay visible.
        if (projectId) {
          const req = requestsById[row.requestId]
          if (req && req.projectId !== projectId) {return false}
        }
        return true
      }),
    [rows, subjectGroup, overdueOnly, projectId, requestsById, clock]
  )

  const decide = useCallback(
    async (method: 'approve' | 'reject', row: Approval, comment?: string): Promise<ApprovalDecisionResult> => {
      // Why: the wire contract requires expectedDigest; never send a decision without it.
      if (!row.subjectDigest) {return { outcome: 'validation' }}
      if (method === 'reject' && [...(comment ?? '').trim()].length < REJECT_REASON_MIN_LENGTH) {
        return { outcome: 'validation', code: 'APPROVAL_COMMENT_REQUIRED' }
      }
      const result = await callRequestRpc(
        method === 'approve' ? REQUEST_RPC_METHODS.APPROVAL_APPROVE : REQUEST_RPC_METHODS.APPROVAL_REJECT,
        {
          id: row.id,
          expectedVersion: row.version,
          expectedDigest: row.subjectDigest,
          ...(comment !== undefined ? { comment } : {})
        }
      )
      if (result.ok) {
        setRows((prev) => prev.filter((r) => r.id !== row.id))
        return { outcome: 'ok' }
      }
      const classified = classifyApprovalDecisionError(result.error)
      // The RPC client already classified; fall back to its kind when the code is not an approval code.
      const outcome: ApprovalDecisionOutcome =
        classified.outcome !== 'unknown' ? classified.outcome
          : result.error.kind === 'network' ? 'network'
            : result.error.kind === 'unsupported' ? 'unsupported' : 'unknown'
      if (outcome === 'closed') {setRows((prev) => prev.filter((r) => r.id !== row.id))}
      if (outcome === 'changed' || outcome === 'forbidden') {refetch()}
      if (outcome === 'unsupported') {setRequestFlowSupport('unsupported')}
      return { outcome, code: classified.code }
    },
    [refetch, setRequestFlowSupport]
  )

  const approve = useCallback((row: Approval) => decide('approve', row), [decide])
  const reject = useCallback((row: Approval, comment: string) => decide('reject', row, comment), [decide])

  return {
    rows: filtered,
    isLoading,
    isLoadingMore,
    error,
    hasMore: nextPageToken !== null,
    loadMore,
    refetch,
    approve,
    reject,
    supported: supportState !== 'unsupported'
  }
}
