/**
 * quality-copy-factory.ts — FE-CV-TASK-087-08
 *
 * Builds a typed `copy(key, params)` function over an English table whose keys are read by name
 * (not auto-extracted), mirroring components/quality-charts/quality-chart-copy. The locale
 * coverage test enumerates the table so non-English UIs never silently fall back to English.
 * Translation happens at call time (never at module scope) so the active locale is honoured.
 *
 * @module components/review-map/quality/quality-copy-factory
 */

import { translate } from '@/i18n/i18n'

export const REVIEW_QUALITY_COPY_ROOT = 'auto.components.reviewQuality.'

export type QualityCopyParams = Record<string, string | number>

export function createQualityCopy<T extends Record<string, string>>(
  group: string,
  table: T
): (key: keyof T & string, params?: QualityCopyParams) => string {
  const prefix = `${REVIEW_QUALITY_COPY_ROOT}${group}.`
  return (key, params) => translate(`${prefix}${key}`, table[key], params)
}

/** Catalog key prefix of a copy group, for locale coverage tests. */
export function qualityCopyPrefix(group: string): string {
  return `${REVIEW_QUALITY_COPY_ROOT}${group}.`
}
