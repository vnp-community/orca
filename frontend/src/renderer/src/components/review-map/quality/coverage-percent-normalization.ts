/**
 * coverage-percent-normalization.ts — FE-CV-TASK-087-15
 *
 * The only place that knows the unit of coverage numbers on the wire. Contract D5 fixed them
 * as ratios 0..1 (0.75 = 75%); the UI shows 0..100. If the unit ever changes, change this file only.
 *
 * @module components/review-map/quality/coverage-percent-normalization
 */

/** Ratio 0..1 -> percent 0..100 (two decimals); null for missing or out-of-range input, never clamped. */
export function toPercent(ratio: number | null | undefined): number | null {
  if (typeof ratio !== 'number' || !Number.isFinite(ratio) || ratio < 0 || ratio > 1) {
    return null
  }
  // Why: 0.07 * 100 is 7.000000000000001; rounding keeps displayed and tested values exact.
  return Math.round(ratio * 10000) / 100
}
