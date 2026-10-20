/**
 * Calls of the review's drawings — FE-OR-T212, T215
 *
 * Revisions, restore, compare, the estimate of a regeneration and the regeneration itself. Each returns the parsed answer or the
 * failure with its `REQUEST_REVIEW_*` code, and none of them throws. An estimate asks the server what a regeneration would do
 * and starts nothing.
 *
 * @module hooks/reviewArtifactCalls
 */

import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  parseArtifactMeta,
  parseArtifactRevisions,
  parseModelDiff,
  parseRegenerateOutcome,
  parseReviewEstimate
} from '../../../shared/outcome-review-response-parsers'
import type {
  ModelDiff,
  RegenerateOutcome,
  ReviewArtifactMeta,
  ReviewEstimate
} from '../../../shared/outcome-review-types'
import type { RequestRpcError } from '../../../shared/request-errors'

export type ReviewCallFailure = { kind: RequestRpcError['kind']; code: string; message: string }
export type ReviewCallResult<T> = { ok: true; value: T } | { ok: false; error: ReviewCallFailure }

function failed(error: RequestRpcError): { ok: false; error: ReviewCallFailure } {
  return {
    ok: false,
    error: { kind: error.kind, code: error.backendCode ?? '', message: error.message }
  }
}

const UNREADABLE: { ok: false; error: ReviewCallFailure } = {
  ok: false,
  error: { kind: 'unknown', code: '', message: 'unreadable answer' }
}

export type DrawingKey = { section: string; variant: string; name?: string }

export async function fetchRevisions(
  requestId: string,
  key: DrawingKey
): Promise<ReviewCallResult<ReviewArtifactMeta[]>> {
  const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_REVISIONS, {
    id: requestId,
    section: key.section,
    variant: key.variant,
    ...(key.name ? { name: key.name } : {})
  })
  return res.ok ? { ok: true, value: parseArtifactRevisions(res.value) } : failed(res.error)
}

export async function compareRevisions(
  requestId: string,
  fromArtifactId: string,
  toArtifactId: string
): Promise<ReviewCallResult<ModelDiff>> {
  const res = await callRequestRpc<{ diff?: unknown }>(REQUEST_RPC_METHODS.REVIEW_COMPARE, {
    id: requestId,
    fromArtifactId,
    toArtifactId
  })
  if (!res.ok) {
    return failed(res.error)
  }
  const diff = parseModelDiff(res.value?.diff)
  return diff ? { ok: true, value: diff } : UNREADABLE
}

export async function restoreRevision(
  requestId: string,
  artifactId: string,
  note: string
): Promise<ReviewCallResult<ReviewArtifactMeta>> {
  const res = await callRequestRpc<{ artifact?: unknown }>(REQUEST_RPC_METHODS.REVIEW_RESTORE, {
    id: requestId,
    artifactId,
    ...(note ? { note } : {})
  })
  if (!res.ok) {
    return failed(res.error)
  }
  const artifact = parseArtifactMeta(res.value?.artifact)
  return artifact ? { ok: true, value: artifact } : UNREADABLE
}

export type RegenerationScope = 'artifact' | 'context' | 'all'
export type RegenerationReason =
  | 'looks_wrong'
  | 'kg_changed'
  | 'code_changed'
  | 'style_changed'
  | 'retry_failed'
  | 'other'

export type RegenerationRequest = {
  scope: RegenerationScope
  /** The sections to draw again; empty means all of them. */
  sections: string[]
}

export async function estimateRegeneration(
  requestId: string,
  req: RegenerationRequest
): Promise<ReviewCallResult<ReviewEstimate>> {
  const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_ESTIMATE, {
    id: requestId,
    scope: req.scope,
    sections: req.sections
  })
  if (!res.ok) {
    return failed(res.error)
  }
  const estimate = parseReviewEstimate(res.value)
  return estimate ? { ok: true, value: estimate } : UNREADABLE
}

export type RegenerateInput = RegenerationRequest & {
  reason: RegenerationReason
  note: string
  confirmCost: boolean
  /** One value per press of the button, so a repeated request is one run. */
  clientRequestId: string
  expectedVersion: number
}

export async function regenerate(
  requestId: string,
  input: RegenerateInput
): Promise<ReviewCallResult<RegenerateOutcome>> {
  // Why not retried automatically: a regeneration spends an agent run, and the key only protects a repeat the server saw.
  const res = await callRequestRpc<unknown>(
    REQUEST_RPC_METHODS.REVIEW_REGENERATE,
    {
      id: requestId,
      scope: input.scope,
      sections: input.sections,
      reason: input.reason,
      ...(input.note ? { note: input.note } : {}),
      ...(input.confirmCost ? { confirmCost: true } : {}),
      clientRequestId: input.clientRequestId,
      expectedVersion: input.expectedVersion
    },
    { idempotent: false }
  )
  return res.ok ? { ok: true, value: parseRegenerateOutcome(res.value) } : failed(res.error)
}
