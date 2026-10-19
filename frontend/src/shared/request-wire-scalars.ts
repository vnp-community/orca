/**
 * Scalar guards of the request wire parsers: they never throw, and a value of the wrong type becomes the documented empty one.
 *
 * @module shared/request-wire-scalars
 */

export function safeEnum<T extends string>(value: unknown, valid: Set<T>, fallback: T): T {
  return typeof value === 'string' && valid.has(value as T) ? (value as T) : fallback
}

export function safeStr(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export function safeOptStr(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined
}

export function safeOptNum(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

export function safeNum(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

export function safeArr<T>(value: unknown, mapper: (item: unknown, i: number) => T): T[] {
  return Array.isArray(value) ? value.map(mapper) : []
}

export function safeRaw(obj: Record<string, unknown>, ...known: string[]): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (!known.includes(k)) {
      result[k] = v
    }
  }
  return result
}
