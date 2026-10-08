/**
 * useRequestSubscription — CR-REQ-018-03
 *
 * Screen-level listener on the request event bus (fed by the single global
 * stream/poll owned by useRequestEvents). Never opens its own socket: each
 * extra stream costs the gateway one ephemeral NATS connection.
 *
 * @module hooks/useRequestSubscription
 */

import { useEffect, useRef } from 'react'
import { subscribeRequestBus } from '../lib/request-event-bus'
import type { RequestEvent } from '../../../shared/request-types'

type UseRequestSubscriptionOptions = {
  /** Only deliver events for this request (poll events with empty id always pass). */
  requestId?: string | null
  onEvent: (event: RequestEvent) => void
}

export function useRequestSubscription({ requestId, onEvent }: UseRequestSubscriptionOptions): void {
  const onEventRef = useRef(onEvent)
  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  useEffect(() => {
    return subscribeRequestBus((event) => {
      if (!requestId || !event.requestId || event.requestId === requestId) {
        onEventRef.current(event)
      }
    })
  }, [requestId])
}
