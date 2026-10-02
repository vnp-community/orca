import { beforeEach, describe, expect, it, vi } from 'vitest'

const track = vi.fn()
vi.mock('./telemetry', () => ({ track: (...a: unknown[]) => track(...a) }))

import { eventSchemas } from '../../../shared/telemetry-events'
import {
  bucketLatencyMs,
  bucketLifetimeDays,
  bucketScopeCount,
  trackMcpApprovalDecided,
  trackMcpConsentDecided,
  trackMcpTokenCreated,
  trackMcpTokenRevoked
} from './mcp-telemetry'

beforeEach(() => track.mockReset())

describe('mcp-telemetry buckets', () => {
  it('buckets at the boundaries', () => {
    expect([0, 1, 2, 3, 9].map(bucketScopeCount)).toEqual(['1', '1', '2', '3+', '3+'])
    expect([1, 7, 8, 30, 31, 90, 91].map(bucketLifetimeDays)).toEqual([
      '<=7d',
      '<=7d',
      '<=30d',
      '<=30d',
      '<=90d',
      '<=90d',
      '>90d'
    ])
    expect([0, 9_999, 10_000, 59_999, 60_000, 599_999, 600_000].map(bucketLatencyMs)).toEqual([
      '<10s',
      '<10s',
      '<60s',
      '<60s',
      '<10m',
      '<10m',
      '>=10m'
    ])
  })
})

describe('mcp-telemetry wrappers emit schema-valid, coarse payloads', () => {
  it('validates every wrapper payload against the registry', () => {
    trackMcpConsentDecided({
      decision: 'approve',
      scopeCount: 4,
      newClient: true,
      viaDcr: false,
      narrowed: true
    })
    trackMcpTokenCreated({ lifetimeDays: 30, scopeCount: 2 })
    trackMcpTokenRevoked()
    trackMcpApprovalDecided({
      decision: 'deny',
      risk: 'destructive',
      via: 'inbox',
      latencyMs: 61_000
    })
    expect(track).toHaveBeenCalledTimes(4)
    for (const [name, props] of track.mock.calls) {
      expect(eventSchemas[name as keyof typeof eventSchemas].safeParse(props).success).toBe(true)
    }
    expect(track.mock.calls[0][1]).toEqual({
      decision: 'approve',
      scope_count: '3+',
      new_client: true,
      via_dcr: false,
      narrowed: true
    })
  })
})
