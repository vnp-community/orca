/**
 * Tests for request-event-bus.ts (CR-REQ-018-02)
 */

import { describe, it, expect, vi } from 'vitest'
import { subscribeRequestBus, emitRequestEvent } from './request-event-bus'
import type { RequestEvent } from '../../../shared/request-types'

function makeEvent(overrides: Partial<RequestEvent> = {}): RequestEvent {
  return {
    requestId: 'r1',
    eventType: 'request.status_changed',
    occurredAt: '2024-01-01T00:00:00Z',
    ...overrides
  }
}

describe('request-event-bus', () => {
  it('delivers events to subscribed listeners', () => {
    const listener = vi.fn()
    const unsub = subscribeRequestBus(listener)
    const event = makeEvent()
    emitRequestEvent(event)
    expect(listener).toHaveBeenCalledWith(event)
    unsub()
  })

  it('stops delivery after unsubscribe', () => {
    const listener = vi.fn()
    const unsub = subscribeRequestBus(listener)
    unsub()
    emitRequestEvent(makeEvent())
    expect(listener).not.toHaveBeenCalled()
  })

  it('delivers to multiple listeners', () => {
    const l1 = vi.fn()
    const l2 = vi.fn()
    const u1 = subscribeRequestBus(l1)
    const u2 = subscribeRequestBus(l2)
    const event = makeEvent()
    emitRequestEvent(event)
    expect(l1).toHaveBeenCalledWith(event)
    expect(l2).toHaveBeenCalledWith(event)
    u1()
    u2()
  })

  it('one throwing listener does not prevent others from receiving', () => {
    const throwing = vi.fn(() => { throw new Error('boom') })
    const safe = vi.fn()
    const u1 = subscribeRequestBus(throwing)
    const u2 = subscribeRequestBus(safe)
    emitRequestEvent(makeEvent())
    expect(safe).toHaveBeenCalled()
    u1()
    u2()
  })
})
