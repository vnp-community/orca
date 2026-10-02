import { describe, expect, it, vi } from 'vitest'
import { emitMcpEvent, subscribeMcpEvents } from './mcp-event-bus'

describe('mcp-event-bus', () => {
  it('isolates listener failures and supports unsubscribe', () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const good = vi.fn()
    const offBad = subscribeMcpEvents(() => {
      throw new Error('x')
    })
    const offGood = subscribeMcpEvents(good)
    emitMcpEvent({ type: 'grant.revoked', grantId: 'g' })
    expect(good).toHaveBeenCalledTimes(1)
    offBad()
    offGood()
    emitMcpEvent({ type: 'grant.revoked', grantId: 'g' })
    expect(good).toHaveBeenCalledTimes(1)
    err.mockRestore()
  })
})
