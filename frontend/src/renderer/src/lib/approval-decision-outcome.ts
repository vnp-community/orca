/**
 * Approval decision outcome — CR-REQ-022-02
 *
 * Maps a failed approve/reject call to what the inbox should do next.
 * Codes are matched by suffix because CR-016 drops the REQUEST_ prefix that
 * CR-009/010 and CONTRACT keep.
 *
 * @module lib/approval-decision-outcome
 */

import { classifyRequestRpcError, splitErrorCode } from '../../../shared/request-errors'

export type ApprovalDecisionOutcome =
  | 'ok' | 'closed' | 'changed' | 'forbidden' | 'validation' | 'unsupported' | 'network' | 'unknown'

const CLOSED = new Set(['APPROVAL_ALREADY_DECIDED', 'APPROVAL_EXPIRED', 'APPROVAL_NOT_FOUND', 'NOT_FOUND'])
const CHANGED = new Set(['APPROVAL_VERSION_CONFLICT', 'APPROVAL_DIGEST_MISMATCH', 'APPROVAL_STAGE_MISMATCH'])
const FORBIDDEN = new Set([
  'APPROVAL_NOT_APPROVER', 'APPROVAL_FORBIDDEN', 'APPROVAL_SELF_APPROVAL_FORBIDDEN',
  'APPROVAL_AGENT_FORBIDDEN', 'FORBIDDEN'
])

function normalizeCode(code: string): string {
  return code.toUpperCase().replace(/^REQUEST_/, '')
}

export function classifyApprovalDecisionError(err: unknown): { outcome: ApprovalDecisionOutcome; code?: string } {
  const rpc = classifyRequestRpcError(err)
  // The classifier keeps the transport code in `code`; the domain code is inside the message.
  const fromMessage = splitErrorCode(rpc.message).code
  const code = normalizeCode(fromMessage || rpc.code)
  if (rpc.kind === 'unsupported') {return { outcome: 'unsupported', code }}
  if (rpc.kind === 'network') {return { outcome: 'network', code }}
  if (CLOSED.has(code)) {return { outcome: 'closed', code }}
  if (CHANGED.has(code)) {return { outcome: 'changed', code }}
  if (FORBIDDEN.has(code)) {return { outcome: 'forbidden', code }}
  if (code === 'APPROVAL_COMMENT_REQUIRED') {return { outcome: 'validation', code }}
  return { outcome: 'unknown', code }
}
