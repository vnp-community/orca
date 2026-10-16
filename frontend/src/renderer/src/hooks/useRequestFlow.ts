/**
 * useRequestFlow — FE-REQ-TASK-037-02
 *
 * The flow (analysis, plan, gates) of one request type and size, read from `request.flow` so the screens follow the backend
 * registry. Until the answer arrives, and for good when the gateway has no such channel, the built-in table answers.
 *
 * @module hooks/useRequestFlow
 */

import { useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { fallbackRequestFlow, parseRequestFlow } from '../../../shared/request-flow-view'
import type { RequestFlowView } from '../../../shared/request-execution-types'
import type { RequestSize, RequestType } from '../../../shared/request-types'

const answers = new Map<string, RequestFlowView>()
const inFlight = new Map<string, Promise<RequestFlowView | null>>()

/** Test hook: forgets what was read so a test starts from the fallback. */
export function resetRequestFlowCache(): void {
  answers.clear()
  inFlight.clear()
}

function keyOf(type: RequestType, size: RequestSize | undefined): string {
  return `${type}|${size ?? ''}`
}

function readFlow(
  type: RequestType,
  size: RequestSize | undefined
): Promise<RequestFlowView | null> {
  const key = keyOf(type, size)
  const known = answers.get(key)
  if (known) {
    return Promise.resolve(known)
  }
  const pending = inFlight.get(key)
  if (pending) {
    return pending
  }
  const call = callRequestRpc<unknown>(REQUEST_RPC_METHODS.FLOW, {
    type,
    ...(size ? { size } : {})
  })
    .then((res) => {
      if (!res.ok) {
        return null
      }
      const flow = parseRequestFlow(res.value)
      if (flow) {
        answers.set(key, flow)
      }
      return flow
    })
    .finally(() => inFlight.delete(key))
  inFlight.set(key, call)
  return call
}

/** The flow for `type` and `size`; null for an unknown type. Never errors: the fallback table is always there. */
export function useRequestFlow(
  type: RequestType | undefined,
  size?: RequestSize
): RequestFlowView | null {
  const initial = (): RequestFlowView | null => {
    if (!type || type === 'unknown') {
      return null
    }
    return answers.get(keyOf(type, size)) ?? fallbackRequestFlow(type, size)
  }
  const [flow, setFlow] = useState<RequestFlowView | null>(initial)

  useEffect(() => {
    setFlow(initial())
    if (!type || type === 'unknown') {
      return undefined
    }
    let cancelled = false
    void readFlow(type, size).then((answer) => {
      if (!cancelled && answer) {
        setFlow(answer)
      }
    })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type, size])

  return flow
}
