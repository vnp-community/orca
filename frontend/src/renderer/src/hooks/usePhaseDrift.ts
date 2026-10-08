/**
 * usePhaseDrift — FE-REQ-TASK-036-07
 *
 * Tasks whose actual change drifted from the plan (`impact.drift {phaseId}`), for
 * the "Drifted" chip on plan rows. Channel name is provisional (CR-030 2.7);
 * unsupported runtimes simply show no chip.
 *
 * @module hooks/usePhaseDrift
 */

import { useCallback, useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseImpactDrift } from '../../../shared/request-artifact-parsers'

const EVENTS = ['impact.drift_detected'] as const
const EMPTY: ReadonlySet<string> = new Set()

export function usePhaseDrift(params: {
  requestId: string | null
  phaseId: string
  enabled: boolean
}): ReadonlySet<string> {
  const { requestId, phaseId, enabled } = params
  const [drifted, setDrifted] = useState<ReadonlySet<string>>(EMPTY)
  const [tick, setTick] = useState(0)
  const refetch = useCallback(() => setTick((n) => n + 1), [])
  useRefetchOnRequestEvent(enabled ? requestId : null, EVENTS, refetch)

  useEffect(() => {
    if (!enabled || !phaseId) {
      setDrifted(EMPTY)
      return
    }
    let cancelled = false
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_DRIFT, { phaseId }).then((res) => {
      if (cancelled) {
        return
      }
      const drift = res.ok
        ? parseImpactDrift((res.value as { drift?: unknown } | null)?.drift ?? res.value)
        : null
      setDrifted(drift && drift.drifted ? new Set(drift.items.map((i) => i.taskId)) : EMPTY)
    })
    return () => {
      cancelled = true
    }
  }, [enabled, phaseId, tick])

  return drifted
}
