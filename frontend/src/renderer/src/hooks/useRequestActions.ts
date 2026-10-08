/**
 * useRequestActions — CR-REQ-018-03
 *
 * Provides mutation actions for the request workflow.
 * Prevents double-submit via an in-flight map keyed by (action, id).
 * Client-side validation for empty reason fields returns a synthetic 'validation' error.
 *
 * @module hooks/useRequestActions
 */

import { useRef, useCallback } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { generatePlanProposeCommit } from './request-plan-generation'
import { parseRequest } from '../../../shared/request-wire-parsers'
import type { Result } from '../runtime/request-rpc-client'
import type { RequestRpcError } from '../../../shared/request-errors'
import type { OrcaRequest, RequestType, RequestSize, RequestUrgency } from '../../../shared/request-types'

function validationError(message: string): Result<never> {
  return {
    ok: false,
    error: { kind: 'validation', code: 'REQUEST_REASON_REQUIRED', message } as RequestRpcError
  }
}

export function useRequestActions() {
  // Map of `${action}:${id}` → true, prevents double-submit
  const inFlight = useRef(new Map<string, boolean>())

  function guard(action: string, id: string): boolean {
    const key = `${action}:${id}`
    if (inFlight.current.get(key)) {return false}
    inFlight.current.set(key, true)
    return true
  }

  function release(action: string, id: string): void {
    inFlight.current.delete(`${action}:${id}`)
  }

  const classify = useCallback(async (id: string): Promise<Result<unknown>> => {
    if (!guard('classify', id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already classifying' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.CLASSIFY, { id }) }
    finally { release('classify', id) }
  }, [])

  const confirmType = useCallback(async (params: {
    id: string
    type: RequestType
    size?: RequestSize
    urgency?: RequestUrgency
    reason?: string
    expectedVersion?: number
  }): Promise<Result<unknown>> => {
    if (!guard('confirmType', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already confirming' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.CONFIRM_TYPE, params) }
    finally { release('confirmType', params.id) }
  }, [])

  const changeType = useCallback(async (params: {
    id: string
    toType: RequestType
    reason: string
    expectedVersion?: number
  }): Promise<Result<unknown>> => {
    if (!params.reason.trim()) {return validationError('Reason is required')}
    if (!guard('changeType', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already changing type' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.CHANGE_TYPE, params) }
    finally { release('changeType', params.id) }
  }, [])

  const returnToBacklog = useCallback(async (params: {
    id: string
    stage: string
    reason: string
  }): Promise<Result<unknown>> => {
    if (!params.reason.trim()) {return validationError('Reason is required')}
    if (!guard('returnToBacklog', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already returning' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.RETURN_TO_BACKLOG, params) }
    finally { release('returnToBacklog', params.id) }
  }, [])

  const reopen = useCallback(async (id: string): Promise<Result<unknown>> => {
    if (!guard('reopen', id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already reopening' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.REOPEN, { id }) }
    finally { release('reopen', id) }
  }, [])

  const cancel = useCallback(async (params: {
    id: string
    reason?: string
  }): Promise<Result<unknown>> => {
    if (!guard('cancel', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already cancelling' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.CANCEL, params) }
    finally { release('cancel', params.id) }
  }, [])

  const spawnChild = useCallback(async (params: {
    id: string
    reason: string
    title: string
    body?: string
    type?: RequestType
    clientRequestId?: string
  }): Promise<Result<{ request: OrcaRequest }>> => {
    if (!guard('spawnChild', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already spawning' } as RequestRpcError }}
    try {
      // Why: CONTRACT names these linkReason/typeHint and answers {child, created}; the
      // draft names (reason/type, {request}) are sent too and the result is normalised.
      const result = await callRequestRpc<{ request?: OrcaRequest; child?: OrcaRequest }>(
        REQUEST_RPC_METHODS.SPAWN_CHILD,
        { ...params, linkReason: params.reason, typeHint: params.type }
      )
      if (!result.ok) {return result}
      const child = result.value.child ?? result.value.request
      return child ? { ok: true, value: { request: parseRequest(child) } } : result as Result<{ request: OrcaRequest }>
    }
    finally { release('spawnChild', params.id) }
  }, [])

  const generatePlan = useCallback(async (id: string): Promise<Result<unknown>> => {
    if (!guard('generatePlan', id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already generating plan' } as RequestRpcError }}
    try { return await generatePlanProposeCommit(id) }
    finally { release('generatePlan', id) }
  }, [])

  const startPhase = useCallback(async (params: {
    id: string
    phaseTaskId?: string
  }): Promise<Result<unknown>> => {
    if (!guard('startPhase', params.id)) {return { ok: false, error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'Already starting phase' } as RequestRpcError }}
    try { return await callRequestRpc(REQUEST_RPC_METHODS.START_PHASE, params) }
    finally { release('startPhase', params.id) }
  }, [])

  return {
    classify,
    confirmType,
    changeType,
    returnToBacklog,
    reopen,
    cancel,
    spawnChild,
    generatePlan,
    startPhase
  }
}
