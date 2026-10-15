/**
 * usePlanRunInFlight
 *
 * Whether the server is writing this Request's Plan right now. The generate button's busy state lives in the browser tab that
 * clicked it, so after a refresh the tab would otherwise offer "Generate plan" while a run is still going.
 *
 * @module hooks/usePlanRunInFlight
 */

import { useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { OrcaRequest } from '../../../shared/request-types'

export const PLAN_RUN_PRESENCE_POLL_MS = 4000

type RunView = { status?: string }

export function usePlanRunInFlight(
  request: Pick<OrcaRequest, 'id' | 'status' | 'planTaskId'>,
  onFinished: () => void
): boolean {
  const [running, setRunning] = useState(false)
  const wasRunning = useRef(false)
  const finished = useRef(onFinished)
  finished.current = onFinished
  const waiting = request.status === 'planning' && !request.planTaskId

  useEffect(() => {
    if (!waiting) {
      wasRunning.current = false
      setRunning(false)
      return
    }
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined
    const tick = async (): Promise<void> => {
      // A Request that never ran a plan answers "run not found": that is "no run", not a failure to show.
      const res = await callRequestRpc<RunView>(REQUEST_RPC_METHODS.PLAN_PROPOSAL, {
        id: request.id
      })
      if (cancelled) {
        return
      }
      const now = res.ok && res.value?.status === 'running'
      setRunning(now)
      if (wasRunning.current && !now) {
        finished.current()
      }
      wasRunning.current = now
      timer = setTimeout(() => void tick(), PLAN_RUN_PRESENCE_POLL_MS)
    }
    void tick()
    return () => {
      cancelled = true
      if (timer) {
        clearTimeout(timer)
      }
    }
  }, [waiting, request.id])

  return waiting && running
}
