/**
 * code-intel-quality-parse-primitives.ts — FE-CV-TASK-087-01
 *
 * Shared never-throw readers for the quality wire parsers.
 *
 * @module shared/code-intel-quality-parse-primitives
 */

import type { GateResult } from './code-intel-quality-types'

export function rec(v: unknown): Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : {}
}

export function list(v: unknown): unknown[] {
  return Array.isArray(v) ? v : []
}

export function str(v: unknown, fallback = ''): string {
  return typeof v === 'string' ? v : fallback
}

export function optStr(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}

export function count(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) && v > 0 ? v : 0
}

export function finiteOrNull(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

export function optFinite(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}

export function oneOf<T extends string>(v: unknown, allowed: ReadonlySet<string>): T | 'unknown' {
  return typeof v === 'string' && allowed.has(v) ? (v as T) : 'unknown'
}

export function strList(v: unknown): string[] {
  return list(v).filter((x): x is string => typeof x === 'string')
}

const GATE_RESULTS = new Set(['pass', 'warn', 'fail'])

export function parseGateResult(raw: unknown): GateResult {
  return oneOf(raw, GATE_RESULTS)
}
