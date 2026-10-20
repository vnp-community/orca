/**
 * Readers shared by the Outcome Review parsers — FE-OR-T111
 *
 * @module shared/outcome-review-wire-readers
 */

import { safeArr, safeStr } from './request-wire-scalars'
import type { Open } from './outcome-review-types'

export function obj(raw: unknown): Record<string, unknown> {
  return raw && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : {}
}

export function bool(v: unknown): boolean {
  return v === true
}

export function strings(v: unknown): string[] {
  return safeArr(v, safeStr).filter((s) => s !== '')
}

export function openEnum<T extends string>(v: unknown): Open<T> {
  return safeStr(v)
}
