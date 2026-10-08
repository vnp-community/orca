/**
 * Request relative time — CR-REQ-022-03
 *
 * Formats timestamps against the app UI language (i18n.language), not the
 * browser locale, in minute-or-coarser units to match the 60 s clock tick.
 *
 * @module lib/request-relative-time
 */

import { i18n } from '@/i18n/i18n'

const UNITS: { unit: Intl.RelativeTimeFormatUnit; ms: number }[] = [
  { unit: 'year', ms: 365 * 24 * 3_600_000 },
  { unit: 'month', ms: 30 * 24 * 3_600_000 },
  { unit: 'day', ms: 24 * 3_600_000 },
  { unit: 'hour', ms: 3_600_000 },
  { unit: 'minute', ms: 60_000 }
]

function parse(iso: string | undefined): number | null {
  if (!iso) {return null}
  const t = Date.parse(iso)
  return Number.isNaN(t) ? null : t
}

function safeLocale(locale: string): string {
  try {
    return Intl.getCanonicalLocales(locale)[0] ?? 'en'
  } catch {
    return 'en'
  }
}

export function formatRelativeTime(iso: string | undefined, now: number, locale: string): string {
  const t = parse(iso)
  if (t === null) {return '-'}
  const diff = t - now
  const rtf = new Intl.RelativeTimeFormat(safeLocale(locale), { numeric: 'auto' })
  for (const { unit, ms } of UNITS) {
    // Why: trunc, not round, so "59 minutes left" never reads as "1 hour".
    if (Math.abs(diff) >= ms) {return rtf.format(Math.trunc(diff / ms), unit)}
  }
  return rtf.format(0, 'minute')
}

export type DueState = { kind: 'none' | 'upcoming' | 'overdue'; text: string }

export function formatDueState(dueAt: string | undefined, now: number, locale: string): DueState {
  const t = parse(dueAt)
  if (t === null) {return { kind: 'none', text: '' }}
  return { kind: t < now ? 'overdue' : 'upcoming', text: formatRelativeTime(dueAt, now, locale) }
}

export function formatAbsoluteTime(iso: string | undefined, locale: string): string {
  const t = parse(iso)
  if (t === null) {return '-'}
  return new Intl.DateTimeFormat(safeLocale(locale), { dateStyle: 'medium', timeStyle: 'short' }).format(t)
}

// Kept for RequestRow / RequestHistoryTab / RequestBacklogBanner (CR-REQ-019): second granularity, UI language.
export function formatRequestRelativeTime(iso: string | undefined, now = Date.now()): string {
  const t = parse(iso)
  if (t === null) {return '-'}
  const diffSec = Math.round((t - now) / 1000)
  const rtf = new Intl.RelativeTimeFormat(safeLocale(i18n.language || 'en'), { numeric: 'auto' })
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ['year', 31_536_000], ['month', 2_592_000], ['day', 86_400], ['hour', 3_600], ['minute', 60]
  ]
  for (const [unit, secs] of units) {
    if (Math.abs(diffSec) >= secs) {return rtf.format(Math.trunc(diffSec / secs), unit)}
  }
  return rtf.format(diffSec, 'second')
}
