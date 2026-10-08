/**
 * Refetch a hook's data when a matching request event arrives on the bus.
 * Events carry no payload, so they only ever trigger a reload.
 *
 * @module hooks/useRefetchOnRequestEvent
 */

import { useEffect, useRef } from 'react'
import { subscribeRequestBus } from '../lib/request-event-bus'

export function useRefetchOnRequestEvent(
  requestId: string | null,
  eventTypes: readonly string[],
  refetch: () => void
): void {
  const refetchRef = useRef(refetch)
  refetchRef.current = refetch
  const typesKey = eventTypes.join('|')

  useEffect(() => {
    if (!requestId) {return}
    return subscribeRequestBus((event) => {
      if (event.requestId !== requestId) {return}
      if (eventTypes.some((t) => event.eventType.endsWith(t))) {refetchRef.current()}
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestId, typesKey])
}
