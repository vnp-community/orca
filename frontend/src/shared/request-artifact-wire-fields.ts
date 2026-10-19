/**
 * Field readers shared by the request-artifact wire parsers. They never throw;
 * wrong-typed input collapses to `undefined`/fallback so callers can drop items.
 *
 * @module shared/request-artifact-wire-fields
 */

export type Rec = Record<string, unknown>

export function rec(v: unknown): Rec | null {
  return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : null
}
export function str(v: unknown): string | undefined {
  return typeof v === 'string' && v !== '' ? v : undefined
}
export function num(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined
}
export function pick(o: Rec, ...keys: string[]): unknown {
  for (const k of keys) {
    if (o[k] !== undefined) {
      return o[k]
    }
  }
  return undefined
}
export function strings(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}
/** Accepts an object/array, or a JSON string; broken JSON stays a raw string. */
export function jsonish(v: unknown): unknown {
  if (typeof v !== 'string') {
    return v
  }
  try {
    return JSON.parse(v)
  } catch {
    return v
  }
}
export function oneOf<T extends string>(v: unknown, allowed: readonly T[], fallback: T): T {
  return typeof v === 'string' && (allowed as readonly string[]).includes(v) ? (v as T) : fallback
}
