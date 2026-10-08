/**
 * code-intel-enum-fallback.ts — enum fallback helper
 * Part of FE-CV-TASK-050-01; re-exported from code-intel-types.ts.
 */

/** Helper to add 'unknown' to an enum union. Parsers use this to widen valid enums (U4). */
export type WithUnknown<T extends string> = T | 'unknown'
