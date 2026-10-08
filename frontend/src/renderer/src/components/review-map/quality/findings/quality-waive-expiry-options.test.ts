import { describe, expect, it } from 'vitest'
import {
  maxWaiveExpiry,
  resolveWaiveExpiry,
  waiveDateInputBounds
} from './quality-waive-expiry-options'

const NOW = new Date('2026-10-07T10:00:00.000Z')

describe('resolveWaiveExpiry', () => {
  it('adds 7/14/30 days in UTC as RFC 3339', () => {
    expect(resolveWaiveExpiry({ kind: 'days', days: 7 }, NOW)).toEqual({
      ok: true,
      expiresAt: '2026-10-14T10:00:00.000Z'
    })
    expect(resolveWaiveExpiry({ kind: 'days', days: 30 }, NOW)).toEqual({
      ok: true,
      expiresAt: '2026-11-06T10:00:00.000Z'
    })
  })

  it('clamps anything beyond 30 days to the limit', () => {
    expect(resolveWaiveExpiry({ kind: 'days', days: 90 }, NOW)).toEqual({
      ok: true,
      expiresAt: maxWaiveExpiry(NOW).toISOString()
    })
    expect(resolveWaiveExpiry({ kind: 'date', date: '2027-01-01' }, NOW)).toEqual({
      ok: true,
      expiresAt: maxWaiveExpiry(NOW).toISOString()
    })
  })

  it('turns a date into the end of that UTC day', () => {
    expect(resolveWaiveExpiry({ kind: 'date', date: '2026-10-20' }, NOW)).toEqual({
      ok: true,
      expiresAt: '2026-10-20T23:59:59.000Z'
    })
  })

  it('rejects empty, malformed and past choices', () => {
    expect(resolveWaiveExpiry(null, NOW)).toEqual({ ok: false, reason: 'invalid' })
    expect(resolveWaiveExpiry({ kind: 'date', date: 'nope' }, NOW)).toEqual({
      ok: false,
      reason: 'invalid'
    })
    expect(resolveWaiveExpiry({ kind: 'days', days: 0 }, NOW)).toEqual({
      ok: false,
      reason: 'invalid'
    })
    expect(resolveWaiveExpiry({ kind: 'date', date: '2026-10-06' }, NOW)).toEqual({
      ok: false,
      reason: 'past'
    })
  })

  it('gives date input bounds', () => {
    expect(waiveDateInputBounds(NOW)).toEqual({ min: '2026-10-07', max: '2026-11-06' })
  })
})
