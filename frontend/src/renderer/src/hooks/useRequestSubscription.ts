/**
 * useRequestSubscription — CR-REQ-018-03
 *
 * Subscribes to real-time request events. On local targets or
 * unsupported runtimes, falls back to polling the request list
 * every `pollIntervalMs` (default 10s).
 *
 * @module hooks/useRequestSubscription
 */

import { useEffect, useRef } from 'react'
import { subscribeRequestEvents } from '../runtime/request-rpc-client'
import { subscribeRequestBus, emitRequestEvent } from '../lib/request-event-bus'
import type { RequestEvent } from '../../../shared/request-types'

type UseRequestSubscriptionOptions = {
  requestId?: string
  onEvent: (event: RequestEvent) => void
  /** Fallback poll interval ms when streaming is unsupported. 0 disables fallback polling. */
  pollIntervalMs?: number
}

export function useRequestSubscription({
  requestId,
  onEvent,
  pollIntervalMs = 10_000
}: UseRequestSubscriptionOptions): void {
  const onEventRef = useRef(onEvent)
  useEffect(() => { onEventRef.current = onEvent }, [onEvent])

  const requestIdRef = useRef(requestId)
  useEffect(() => { requestIdRef.current = requestId }, [requestId])

  useEffect(() => {
    let pollTimer: ReturnType<typeof setInterval> | null = null

    // Subscribe via the event bus (other hooks can also emit to this bus)
    const busUnsub = subscribeRequestBus((event) => {
      if (!requestIdRef.current || event.requestId === requestIdRef.current) {
        onEventRef.current(event)
      }
    })

    function onFallback() {
      if (pollIntervalMs <= 0) return
      // Emit a synthetic 'refresh' event so the caller can trigger a refetch
      pollTimer = setInterval(() => {
        onEventRef.current({
          requestId: requestIdRef.current ?? '',
          eventType: 'orca.request.poll',
          occurredAt: new Date().toISOString()
        })
      }, pollIntervalMs)
    }

    // Subscribe to streaming events; route to event bus so all subscribers hear them
    const cleanupStream = subscribeRequestEvents({
      requestId,
      onEvent: (event) => {
        emitRequestEvent(event)
      },
      onFallback
    })

    return () => {
      busUnsub()
      cleanupStream()
      if (pollTimer !== null) clearInterval(pollTimer)
    }
  // requestId changes should restart the subscription
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestId, pollIntervalMs])
}
