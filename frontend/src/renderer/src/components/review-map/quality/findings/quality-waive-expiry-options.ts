/**
 * quality-waive-expiry-options.ts — FE-CV-TASK-087-14
 *
 * Expiry choices of a waiver: 7/14/30 days or a calendar date, never later than 30 days from now
 * (contract: CODEINTEL_WAIVER_EXPIRY_INVALID otherwise). Calculated in UTC, returned as RFC 3339.
 *
 * @module components/review-map/quality/findings/quality-waive-expiry-options
 */

export const WAIVE_MAX_DAYS = 30
export const WAIVE_EXPIRY_DAY_CHOICES = [7, 14, 30] as const

const DAY_MS = 24 * 60 * 60 * 1000

export type WaiveExpiryChoice = { kind: 'days'; days: number } | { kind: 'date'; date: string }

export type WaiveExpiryResult =
  | { ok: true; expiresAt: string }
  | { ok: false; reason: 'invalid' | 'past' }

/** Latest instant the backend accepts. */
export function maxWaiveExpiry(now: Date): Date {
  return new Date(now.getTime() + WAIVE_MAX_DAYS * DAY_MS)
}

/** `YYYY-MM-DD` (UTC) bounds for a date input. */
export function waiveDateInputBounds(now: Date): { min: string; max: string } {
  return {
    min: now.toISOString().slice(0, 10),
    max: maxWaiveExpiry(now).toISOString().slice(0, 10)
  }
}

export function resolveWaiveExpiry(choice: WaiveExpiryChoice | null, now: Date): WaiveExpiryResult {
  if (!choice) {
    return { ok: false, reason: 'invalid' }
  }
  const limit = maxWaiveExpiry(now).getTime()
  let target: number
  if (choice.kind === 'days') {
    if (!Number.isFinite(choice.days) || choice.days < 1) {
      return { ok: false, reason: 'invalid' }
    }
    target = now.getTime() + choice.days * DAY_MS
  } else {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(choice.date)) {
      return { ok: false, reason: 'invalid' }
    }
    // Why: a date means "through the end of that UTC day".
    target = Date.parse(`${choice.date}T23:59:59.000Z`)
    if (Number.isNaN(target)) {
      return { ok: false, reason: 'invalid' }
    }
  }
  if (target <= now.getTime()) {
    return { ok: false, reason: 'past' }
  }
  return { ok: true, expiresAt: new Date(Math.min(target, limit)).toISOString() }
}
