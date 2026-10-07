/**
 * useQualityFeatureFlags.ts — FE-CV-TASK-085-01
 *
 * Aggregated feature flag hook for code-intel / quality / AI.
 * Reads from the code-intel slice (050) — does NOT call RPCs directly.
 *
 * Fail-closed: when support state is 'unknown', all flags are false.
 *
 * @module hooks/useQualityFeatureFlags
 */

import { useAppStore } from '@/store'

export type QualityFeatureFlags = {
  /** Overall state of the code-intel settings probe */
  state: 'unknown' | 'enabled' | 'disabled' | 'unsupported'
  /** Code-intel is ready and enabled */
  codeIntel: boolean
  /** Quality gate feature is enabled (requires codeIntel) */
  quality: boolean
  /** AI review feature is enabled (requires quality) */
  ai: boolean
}

const CLOSED_FLAGS: QualityFeatureFlags = {
  state: 'unknown',
  codeIntel: false,
  quality: false,
  ai: false,
}

/**
 * Returns aggregated feature flags.
 * All flags are false when state === 'unknown' or === 'disabled' or === 'unsupported'.
 * Does NOT call codeIntelClient directly.
 *
 * Note: relies on slice selectors written by FE-CV-TASK-050-10/12.
 * When that slice is absent, falls back to fail-closed defaults.
 */
export function useQualityFeatureFlags(): QualityFeatureFlags {
  return useAppStore((state) => {
    // Read from code-intel slice (added by 050-10/12)
    // The selector path may not exist yet if 050-10 is not done;
    // optional chaining provides a safe fallback until then.
    const support = (state as Record<string, unknown>).codeIntelSupportState as
      | { state: string; effective?: { codeIntelEnabled?: boolean; qualityGateEnabled?: boolean; aiReviewEnabled?: boolean } }
      | undefined

    if (!support) {
      // 050-10 slice not present yet → fail closed
      return CLOSED_FLAGS
    }

    const { state: supportState, effective } = support

    if (supportState !== 'enabled' || !effective) {
      return {
        state: supportState as QualityFeatureFlags['state'],
        codeIntel: false,
        quality: false,
        ai: false,
      }
    }

    const codeIntel = effective.codeIntelEnabled === true
    const quality = codeIntel && effective.qualityGateEnabled === true
    const ai = quality && effective.aiReviewEnabled === true

    return {
      state: 'enabled',
      codeIntel,
      quality,
      ai,
    }
  })
}
