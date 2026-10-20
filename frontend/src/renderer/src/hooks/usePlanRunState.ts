/**
 * usePlanRunState
 *
 * What the server is doing about this Request's Plan right now: the latest Plan run and, with CR-REQ-044, the automatic job.
 * Read from the server on every poll rather than remembered by the tab that clicked, so a refreshed page, another tab and a
 * run the server started on its own all look the same.
 *
 * @module hooks/usePlanRunState
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parsePlanRunView, type PlanRunView } from '../../../shared/request-plan-auto'
import { isPlanRunBusy } from '../components/request/plan/plan-auto-display'
import type { OrcaRequest } from '../../../shared/request-types'

export const PLAN_RUN_STATE_POLL_MS = 4000
/** How long "Orca is starting the plan" may stand with no job and no run before the manual button comes back. */
export const PLAN_AUTO_START_GRACE_MS = 20_000

export type PlanRunState = {
  /** The latest run as the server reports it; null when there is none or the read failed. */
  view: PlanRunView | null
  /** The server is writing (or about to save) the Plan. */
  busy: boolean
  graceElapsed: boolean
  /** Read the state now instead of waiting for the next poll. */
  refresh: () => void
}

type Snapshot = { requestId: string; view: PlanRunView | null; graceElapsed: boolean }
const EMPTY_SNAPSHOT: Snapshot = { requestId: '', view: null, graceElapsed: false }

/** Automatic generation is on and nothing has been created yet: the stretch the start grace period covers. */
function awaitsAutoStart(view: PlanRunView | null): boolean {
  const auto = view?.autoPlan
  return !!auto?.enabled && auto.state === '' && !view?.runId && !view?.status
}

export function usePlanRunState(
  request: Pick<OrcaRequest, 'id' | 'status'>,
  onFinished: () => void
): PlanRunState {
  const [snapshot, setSnapshot] = useState<Snapshot>(EMPTY_SNAPSHOT)
  const finished = useRef(onFinished)
  finished.current = onFinished
  const pollNow = useRef<() => void>(() => undefined)
  const wasBusy = useRef(false)
  const active = request.status === 'planning'

  useEffect(() => {
    wasBusy.current = false
    pollNow.current = () => undefined
    if (!active) {
      return undefined
    }
    let cancelled = false
    let inFlight = false
    let again = false
    let poll: ReturnType<typeof setTimeout> | undefined
    let grace: ReturnType<typeof setTimeout> | undefined
    // Why: state is keyed by Request id, so another Request never shows this one's answer.
    const update = (patch: Partial<Omit<Snapshot, 'requestId'>>): void =>
      setSnapshot((prev) =>
        prev.requestId === request.id
          ? { ...prev, ...patch }
          : { ...EMPTY_SNAPSHOT, requestId: request.id, ...patch }
      )

    const trackStart = (next: PlanRunView | null): void => {
      if (!awaitsAutoStart(next)) {
        clearTimeout(grace)
        grace = undefined
        update({ view: next, graceElapsed: false })
      } else if (!grace) {
        grace = setTimeout(() => update({ graceElapsed: true }), PLAN_AUTO_START_GRACE_MS)
        update({ view: next, graceElapsed: false })
      } else {
        update({ view: next })
      }
    }

    const tick = async (): Promise<void> => {
      clearTimeout(poll)
      inFlight = true
      // A Request that never ran a plan answers "run not found": that is "no run", not a failure to show.
      const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.PLAN_PROPOSAL, {
        id: request.id
      })
      inFlight = false
      if (cancelled) {
        return
      }
      const next = res.ok ? parsePlanRunView(res.value) : null
      trackStart(next)
      const busy = isPlanRunBusy(next)
      if (wasBusy.current && !busy) {
        finished.current()
      }
      wasBusy.current = busy
      if (again) {
        again = false
        void tick()
        return
      }
      poll = setTimeout(() => void tick(), PLAN_RUN_STATE_POLL_MS)
    }

    pollNow.current = () => {
      if (inFlight) {
        // Why: the read in flight may predate the action that asked for this one.
        again = true
        return
      }
      void tick()
    }
    void tick()
    return () => {
      cancelled = true
      clearTimeout(poll)
      clearTimeout(grace)
      update({ view: null, graceElapsed: false })
    }
  }, [active, request.id])

  const refresh = useCallback(() => pollNow.current(), [])
  const current = active && snapshot.requestId === request.id ? snapshot : EMPTY_SNAPSHOT
  return {
    view: current.view,
    busy: isPlanRunBusy(current.view),
    graceElapsed: current.graceElapsed,
    refresh
  }
}
