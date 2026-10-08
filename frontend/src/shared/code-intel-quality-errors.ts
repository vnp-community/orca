/**
 * code-intel-quality-errors.ts — FE-CV-TASK-087-01
 *
 * Typed readers for the JSON suffix of quality error messages (`CODEINTEL_X: text | {json}`,
 * contract §2.3). The generic splitter lives in code-intel-error-codes; these narrow the data.
 * Corrupt or missing data yields undefined, never a throw.
 *
 * @module shared/code-intel-quality-errors
 */

import type { MissingCheck } from './code-intel-quality-types'

type ErrorData = Record<string, unknown> | null | undefined

export type EnvNotReadyData = { missing: MissingCheck[]; reason?: string }
export type RunInProgressData = { runId: string; reason?: string }
export type ProfileUnknownData = { available: string[] }
export type WaiverExpiryData = { maxDays: number }

function strings(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}

export function parseEnvNotReadyData(data: ErrorData): EnvNotReadyData | undefined {
  if (!data || !Array.isArray(data.missing)) {
    return undefined
  }
  const missing = data.missing.map((m): MissingCheck => {
    // Why: older backends send plain names; keep them as checks without a reason.
    if (typeof m === 'string') {
      return { check: m, reason: '' }
    }
    const r = (typeof m === 'object' && m !== null ? m : {}) as Record<string, unknown>
    return {
      check: typeof r.check === 'string' ? r.check : '',
      reason: typeof r.reason === 'string' ? r.reason : '',
      ...(typeof r.hint === 'string' ? { hint: r.hint } : {})
    }
  })
  return { missing, ...(typeof data.reason === 'string' ? { reason: data.reason } : {}) }
}

export function parseRunInProgressData(data: ErrorData): RunInProgressData | undefined {
  if (!data || typeof data.runId !== 'string' || data.runId === '') {
    return undefined
  }
  return { runId: data.runId, ...(typeof data.reason === 'string' ? { reason: data.reason } : {}) }
}

export function parseProfileUnknownData(data: ErrorData): ProfileUnknownData | undefined {
  if (!data || !Array.isArray(data.available)) {
    return undefined
  }
  return { available: strings(data.available) }
}

export function parseWaiverExpiryData(data: ErrorData): WaiverExpiryData | undefined {
  const v = data?.maxDays
  return typeof v === 'number' && Number.isFinite(v) && v > 0 ? { maxDays: v } : undefined
}

/** Seconds to wait before retrying; reads `retryAfterSeconds` or `retryAfterMs`. */
export function parseRetryAfter(data: ErrorData): number | undefined {
  if (!data) {
    return undefined
  }
  if (typeof data.retryAfterSeconds === 'number' && data.retryAfterSeconds > 0) {
    return data.retryAfterSeconds
  }
  if (typeof data.retryAfterMs === 'number' && data.retryAfterMs > 0) {
    return Math.ceil(data.retryAfterMs / 1000)
  }
  return undefined
}
