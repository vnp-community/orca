/**
 * useApprovals — CR-REQ-018-03
 *
 * Fetches pending or request-specific approvals and provides approve/reject actions.
 * Client-side validation: reject comment must be >= 10 trimmed chars.
 *
 * @module hooks/useApprovals
 */

import { useState, useEffect, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { parseApproval } from '../../../shared/request-wire-parsers'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { useAppStore } from '../store'
import { REJECT_REASON_MIN_LENGTH } from '../components/request/solution/solution-view-model'
import type { Result } from '../runtime/request-rpc-client'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { Approval } from '../../../shared/request-types'

export type ApprovalFilters = {
  requestId?: string
  status?: string
}

type UseApprovalsResult = {
  approvals: Approval[]
  pendingCount: number
  isLoading: boolean
  error: string | null
  approve: (params: { approval: Approval; comment?: string; viewedImpactDigest?: string; acceptedFindingIds?: string[] }) => Promise<Result<unknown>>
  reject: (params: { approval: Approval; comment: string }) => Promise<Result<unknown>>
  refetch: () => void
}

export function useApprovals(filters: ApprovalFilters = {}): UseApprovalsResult {
  const setPendingApprovalCount = useAppStore((s) => s.setPendingApprovalCount)

  const [approvals, setApprovals] = useState<Approval[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [refetchTrigger, setRefetchTrigger] = useState(0)

  const refetch = useCallback(() => setRefetchTrigger((n) => n + 1), [])

  useEffect(() => {
    let cancelled = false
    setIsLoading(true)
    setError(null)

    const method = filters.requestId
      ? REQUEST_RPC_METHODS.APPROVAL_LIST
      : REQUEST_RPC_METHODS.APPROVAL_LIST_PENDING

    callRequestRpc<{ approvals: unknown[] }>(method, filters).then((result) => {
      if (cancelled) {return}
      setIsLoading(false)

      if (!result.ok) {
        setError(result.error.kind)
        return
      }

      const parsed = (result.value.approvals ?? []).map(parseApproval)
      setApprovals(parsed)

      // Update global pending count from the list
      // Why: only the global pending list is the sidebar badge; a per-request list must not overwrite it.
      if (!filters.requestId) {setPendingApprovalCount(parsed.filter((a) => a.status === 'pending').length)}
    })

    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters.requestId, refetchTrigger])

  const approve = useCallback(async (params: {
    approval: Approval
    comment?: string
    /** Impact digest the approver reviewed (medium and above; names provisional, CR-REQ-030 section 8). */
    viewedImpactDigest?: string
    acceptedFindingIds?: string[]
  }): Promise<Result<unknown>> => {
    return callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_APPROVE, {
      id: params.approval.id,
      approvalId: params.approval.id,
      expectedVersion: params.approval.version,
      expectedDigest: params.approval.subjectDigest,
      comment: params.comment,
      ...(params.viewedImpactDigest ? { viewedImpactDigest: params.viewedImpactDigest } : {}),
      ...(params.acceptedFindingIds && params.acceptedFindingIds.length > 0 ? { acceptedFindingIds: params.acceptedFindingIds } : {})
    })
  }, [])

  const reject = useCallback(async (params: {
    approval: Approval
    comment: string
  }): Promise<Result<unknown>> => {
    // Client-side validation: comment must have meaningful content
    if ([...params.comment.trim()].length < REJECT_REASON_MIN_LENGTH) {
      return {
        ok: false,
        error: {
          kind: 'validation',
          code: 'APPROVAL_COMMENT_REQUIRED',
          message: `Comment must be at least ${REJECT_REASON_MIN_LENGTH} characters`
        } as RequestRpcError
      }
    }
    return callRequestRpc(REQUEST_RPC_METHODS.APPROVAL_REJECT, {
      id: params.approval.id,
      approvalId: params.approval.id,
      expectedVersion: params.approval.version,
      expectedDigest: params.approval.subjectDigest,
      comment: params.comment
    })
  }, [])

  const pendingCount = approvals.filter((a) => a.status === 'pending').length

  return { approvals, pendingCount, isLoading, error, approve, reject, refetch }
}
