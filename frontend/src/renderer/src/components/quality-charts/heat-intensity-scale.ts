export type IntensityBucket = 1 | 2 | 3 | 4 | 5

function toBucket(fraction: number): IntensityBucket {
  return (Math.min(4, Math.max(0, Math.floor(fraction * 5))) + 1) as IntensityBucket
}

/** Even five-way split of the domain; null/NaN has no bucket so the cell shows a dash, not 0. */
export function bucketIntensity(
  value: number | null | undefined,
  domain: { min: number; max: number }
): IntensityBucket | null {
  if (value === null || value === undefined || !Number.isFinite(value)) {
    return null
  }
  if (!Number.isFinite(domain.min) || !Number.isFinite(domain.max) || domain.max <= domain.min) {
    return 3
  }
  const clamped = Math.min(domain.max, Math.max(domain.min, value))
  return toBucket((clamped - domain.min) / (domain.max - domain.min))
}

/** Rank-based buckets (mid-rank for ties), so equal values always share a bucket. */
export function bucketIntensityByQuantile(
  values: readonly (number | null | undefined)[]
): (IntensityBucket | null)[] {
  const numbers = values.filter((v): v is number => typeof v === 'number' && Number.isFinite(v))
  const n = numbers.length
  return values.map((v) => {
    if (typeof v !== 'number' || !Number.isFinite(v)) {
      return null
    }
    if (n <= 1) {
      return 3
    }
    let less = 0
    let equal = 0
    for (const x of numbers) {
      if (x < v) {
        less++
      } else if (x === v) {
        equal++
      }
    }
    return toBucket((less + (equal - 1) / 2) / (n - 1))
  })
}
