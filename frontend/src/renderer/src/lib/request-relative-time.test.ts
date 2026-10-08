import { beforeEach, describe, expect, it, vi } from 'vitest'

const lang = vi.hoisted(() => ({ value: 'en' as string }))
vi.mock('@/i18n/i18n', () => ({
  i18n: {
    get language() {
      return lang.value
    }
  }
}))

import {
  formatAbsoluteTime,
  formatDueState,
  formatRelativeTime,
  formatRequestRelativeTime
} from './request-relative-time'

const NOW = Date.parse('2026-10-07T12:00:00Z')
const at = (offsetMs: number): string => new Date(NOW + offsetMs).toISOString()
const MIN = 60_000
const HOUR = 3_600_000
const DAY = 24 * HOUR

beforeEach(() => {
  lang.value = 'en'
})

describe('formatRelativeTime', () => {
  it('formats the future and the past', () => {
    expect(formatRelativeTime(at(3 * HOUR), NOW, 'en')).toBe('in 3 hours')
    expect(formatRelativeTime(at(-2 * DAY), NOW, 'en')).toBe('2 days ago')
    expect(formatRelativeTime(at(5 * MIN), NOW, 'en')).toBe('in 5 minutes')
    expect(formatRelativeTime(at(-5 * MIN), NOW, 'en')).toBe('5 minutes ago')
  })

  it.each([
    [59 * MIN + 59_000, 'in 59 minutes'],
    [HOUR, 'in 1 hour'],
    [23 * HOUR + 59 * MIN, 'in 23 hours'],
    [DAY, 'tomorrow'],
    [29 * DAY, 'in 29 days'],
    [30 * DAY, 'next month'],
    [365 * DAY, 'next year']
  ])('picks the unit at the boundary (+%i ms)', (offset, expected) => {
    expect(formatRelativeTime(at(offset), NOW, 'en')).toBe(expected)
  })

  it.each([
    [-(HOUR - 1000), '59 minutes ago'],
    [-HOUR, '1 hour ago'],
    [-DAY, 'yesterday'],
    [-30 * DAY, 'last month'],
    [-365 * DAY, 'last year']
  ])('picks the unit at the boundary (%i ms)', (offset, expected) => {
    expect(formatRelativeTime(at(offset), NOW, 'en')).toBe(expected)
  })

  it('truncates toward zero so 59 minutes never reads as one hour', () => {
    expect(formatRelativeTime(at(59 * MIN + 30_000), NOW, 'en')).toBe('in 59 minutes')
    expect(formatRelativeTime(at(-(59 * MIN + 30_000)), NOW, 'en')).toBe('59 minutes ago')
    expect(formatRelativeTime(at(2 * HOUR - 1), NOW, 'en')).toBe('in 1 hour')
  })

  it('reports "this minute" below one minute in either direction, and at zero', () => {
    expect(formatRelativeTime(at(0), NOW, 'en')).toBe('this minute')
    expect(formatRelativeTime(at(59_999), NOW, 'en')).toBe('this minute')
    expect(formatRelativeTime(at(-59_999), NOW, 'en')).toBe('this minute')
  })

  it('uses the supplied locale', () => {
    expect(formatRelativeTime(at(3 * HOUR), NOW, 'es')).toBe('dentro de 3 horas')
    expect(formatRelativeTime(at(-3 * DAY), NOW, 'es')).toBe('hace 3 días')
    expect(formatRelativeTime(at(3 * HOUR), NOW, 'ja')).toBe('3 時間後')
  })

  it('returns "-" for missing or unparsable input', () => {
    expect(formatRelativeTime(undefined, NOW, 'en')).toBe('-')
    expect(formatRelativeTime('', NOW, 'en')).toBe('-')
    expect(formatRelativeTime('nonsense', NOW, 'en')).toBe('-')
  })

  it('falls back to English for invalid locale tags instead of throwing', () => {
    expect(formatRelativeTime(at(3 * HOUR), NOW, 'not_a_locale!!')).toBe('in 3 hours')
    expect(formatRelativeTime(at(3 * HOUR), NOW, '')).toBe('in 3 hours')
  })

  it('is driven by the passed now, not the wall clock', () => {
    const iso = '2020-01-01T00:00:00Z'
    expect(formatRelativeTime(iso, Date.parse(iso) - 2 * HOUR, 'en')).toBe('in 2 hours')
    expect(formatRelativeTime(iso, Date.parse(iso) + 2 * HOUR, 'en')).toBe('2 hours ago')
  })
})

describe('formatDueState', () => {
  it('returns none with empty text when there is no due date or it is invalid', () => {
    expect(formatDueState(undefined, NOW, 'en')).toEqual({ kind: 'none', text: '' })
    expect(formatDueState('', NOW, 'en')).toEqual({ kind: 'none', text: '' })
    expect(formatDueState('garbage', NOW, 'en')).toEqual({ kind: 'none', text: '' })
  })

  it('is upcoming for future dates', () => {
    expect(formatDueState(at(DAY), NOW, 'en')).toEqual({ kind: 'upcoming', text: 'tomorrow' })
  })

  it('is overdue for past dates', () => {
    expect(formatDueState(at(-2 * DAY), NOW, 'en')).toEqual({ kind: 'overdue', text: '2 days ago' })
  })

  it('treats exactly-now as upcoming and one millisecond past as overdue', () => {
    expect(formatDueState(at(0), NOW, 'en').kind).toBe('upcoming')
    expect(formatDueState(at(-1), NOW, 'en').kind).toBe('overdue')
  })

  it('localizes the text', () => {
    expect(formatDueState(at(3 * HOUR), NOW, 'es').text).toBe('dentro de 3 horas')
  })
})

describe('formatAbsoluteTime', () => {
  it('formats a medium date with a short time', () => {
    expect(formatAbsoluteTime('2026-10-07T12:00:00Z', 'en')).toMatch(/Oct 7, 2026/)
    expect(formatAbsoluteTime('2026-10-07T12:00:00Z', 'en')).toMatch(/\d{1,2}:\d{2}/)
  })

  it('uses the locale', () => {
    expect(formatAbsoluteTime('2026-10-07T12:00:00Z', 'es')).toMatch(/oct/i)
  })

  it('returns "-" for missing or invalid input', () => {
    expect(formatAbsoluteTime(undefined, 'en')).toBe('-')
    expect(formatAbsoluteTime('x', 'en')).toBe('-')
  })

  it('does not throw for an invalid locale', () => {
    expect(formatAbsoluteTime('2026-10-07T12:00:00Z', 'not_a_locale!!')).toMatch(/2026/)
  })
})

describe('formatRequestRelativeTime', () => {
  it('keeps second granularity under a minute', () => {
    expect(formatRequestRelativeTime(at(-30_000), NOW)).toBe('30 seconds ago')
    expect(formatRequestRelativeTime(at(45_000), NOW)).toBe('in 45 seconds')
    expect(formatRequestRelativeTime(at(0), NOW)).toBe('now')
  })

  it.each([
    [-(MIN), '1 minute ago'],
    [-(59 * MIN + 59_000), '59 minutes ago'],
    [-HOUR, '1 hour ago'],
    [-DAY, 'yesterday'],
    [-2 * DAY, '2 days ago'],
    [-30 * DAY, 'last month'],
    [-365 * DAY, 'last year'],
    [3 * HOUR, 'in 3 hours'],
    [DAY, 'tomorrow']
  ])('crosses unit boundaries (%i ms)', (offset, expected) => {
    expect(formatRequestRelativeTime(at(offset), NOW)).toBe(expected)
  })

  it('follows the UI language (i18n.language), not the browser locale', () => {
    lang.value = 'es'
    expect(formatRequestRelativeTime(at(-3 * DAY), NOW)).toBe('hace 3 días')
  })

  it('falls back to English when the UI language is empty or invalid', () => {
    lang.value = ''
    expect(formatRequestRelativeTime(at(-2 * DAY), NOW)).toBe('2 days ago')
    lang.value = 'not_a_locale!!'
    expect(formatRequestRelativeTime(at(-2 * DAY), NOW)).toBe('2 days ago')
  })

  it('returns "-" for missing or invalid input', () => {
    expect(formatRequestRelativeTime(undefined, NOW)).toBe('-')
    expect(formatRequestRelativeTime('garbage', NOW)).toBe('-')
  })

  it('defaults now to the current time', () => {
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
    try {
      expect(formatRequestRelativeTime(at(-2 * DAY))).toBe('2 days ago')
    } finally {
      vi.useRealTimers()
    }
  })
})
