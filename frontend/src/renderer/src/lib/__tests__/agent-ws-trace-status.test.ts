import { describe, it, expect } from 'vitest'
import { latestAgentWsStatusForDevServer } from '../agent-ws-trace-status'
import type { TraceEvent } from '../../../../shared/trace'

function makeEvent(overrides: Partial<TraceEvent> & { flow: string }): TraceEvent {
  return {
    id: 'evt-1',
    level: 'ok',
    fields: {},
    ts: 1000,
    ...overrides
  }
}

describe('latestAgentWsStatusForDevServer', () => {
  it('returns null when no event matches any agentWs/agentToken flow prefix', () => {
    const events: TraceEvent[] = [
      makeEvent({ flow: 'devServer:browseDir', fields: { devServerId: 'ds-1' } }),
      makeEvent({ flow: 'terminal:spawn', fields: { devServerId: 'ds-1' } })
    ]

    expect(latestAgentWsStatusForDevServer(events, 'ds-1')).toBeNull()
  })

  it('returns null when the matching flow belongs to a different devServerId', () => {
    const events: TraceEvent[] = [
      makeEvent({ flow: 'agentWs:handshake', fields: { devServerId: 'ds-other' } })
    ]

    expect(latestAgentWsStatusForDevServer(events, 'ds-1')).toBeNull()
  })

  it('returns the most recent matching event when multiple exist for the same devServerId', () => {
    const events: TraceEvent[] = [
      makeEvent({
        id: 'e1',
        flow: 'agentWs:handshake',
        level: 'start',
        ts: 1000,
        fields: { devServerId: 'ds-1' }
      }),
      makeEvent({
        id: 'e2',
        flow: 'agentWs:handshake',
        level: 'ok',
        ts: 2000,
        fields: { devServerId: 'ds-1' }
      })
    ]

    const result = latestAgentWsStatusForDevServer(events, 'ds-1')
    expect(result).toEqual({ flow: 'agentWs:handshake', level: 'ok', ts: 2000, reason: undefined })
  })

  it('recognizes all three agent-ws flow prefixes (agentWs:, agentToken:, agent:tokenManager)', () => {
    const prefixes = ['agentWs:handshake', 'agentToken:issue', 'agent:tokenManager:revoke']

    for (const flow of prefixes) {
      const events: TraceEvent[] = [makeEvent({ flow, fields: { devServerId: 'ds-1' } })]
      const result = latestAgentWsStatusForDevServer(events, 'ds-1')
      expect(result?.flow).toBe(flow)
    }
  })

  // Regression test for a live incident (2026-09-13): api-gateway forwards
  // raw NATS `orca.*.trace.span` payload bytes to every connected browser
  // with no shape validation (trace_broadcast.go) — a malformed event
  // missing `flow` crashed the whole Settings > Dev Servers page with
  // "Cannot read properties of undefined (reading 'startsWith')".
  it('does not throw and skips events with a missing/non-string flow', () => {
    const malformed = { id: 'evt-bad', level: 'ok', fields: { devServerId: 'ds-1' }, ts: 500 }
    const events = [
      malformed,
      makeEvent({ flow: 'devServer:browseDir', fields: { devServerId: 'ds-1' } })
    ] as TraceEvent[]

    expect(() => latestAgentWsStatusForDevServer(events, 'ds-1')).not.toThrow()
    expect(latestAgentWsStatusForDevServer(events, 'ds-1')).toBeNull()
  })

  it('surfaces the reason field when level is fail', () => {
    const events: TraceEvent[] = [
      makeEvent({
        flow: 'agentWs:handshake',
        level: 'fail',
        fields: { devServerId: 'ds-1', reason: 'token expired' }
      })
    ]

    const result = latestAgentWsStatusForDevServer(events, 'ds-1')
    expect(result).toEqual({
      flow: 'agentWs:handshake',
      level: 'fail',
      ts: 1000,
      reason: 'token expired'
    })
  })
})
