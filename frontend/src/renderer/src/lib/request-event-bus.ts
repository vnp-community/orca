/**
 * Request Event Bus — CR-REQ-018-02
 *
 * Simple typed event bus for request events. Mirrors mcp-event-bus.ts
 * pattern: Set of listeners, try/catch per listener so one broken
 * listener doesn't prevent others from receiving events.
 *
 * @module lib/request-event-bus
 */

import type { RequestEvent } from '../../../shared/request-types'

type RequestEventListener = (event: RequestEvent) => void

const listeners = new Set<RequestEventListener>()

/** Subscribe to request events. Returns an unsubscribe function. */
export function subscribeRequestBus(listener: RequestEventListener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Emit a request event to all subscribers. Per-listener errors are caught. */
export function emitRequestEvent(event: RequestEvent): void {
  for (const listener of Array.from(listeners)) {
    try {
      listener(event)
    } catch (err) {
      // One broken listener must not starve the others.
      console.error('[request-event-bus] listener failed', err)
    }
  }
}
