// Cohort discriminator for onboarding-wizard telemetry events. Ported
// verbatim from desktop/src/main/telemetry/onboarding-cohort-classifier.ts —
// no Electron dependency, pure logic over `Store`. See that file's header
// comment for the full fresh_install/upgrade_backfill cohort design and its
// documented limitation (mirrors here unchanged).
//
// Failure mode: this module never throws. On any read error or
// store-not-yet-initialized condition, `getOnboardingCohortAtEmit` returns
// `{ cohort: undefined }`. Mirrors `getCohortAtEmit`.

import { ONBOARDING_FINAL_STEP } from '../../shared/constants'
import type { OnboardingCohort } from '../../shared/telemetry-events'
import type { Store } from '../persistence'

let storeRef: Store | null = null

let warnedThisSession = false

export function initOnboardingCohortClassifier(store: Store): void {
  storeRef = store
  warnedThisSession = false
}

export function getOnboardingCohortAtEmit(): { cohort: OnboardingCohort | undefined } {
  if (!storeRef) {
    warnOnce('store not initialized')
    return { cohort: undefined }
  }
  try {
    const settings = storeRef.getSettings()
    const existedBefore = settings.telemetry?.existedBeforeTelemetryRelease
    if (existedBefore === false) {
      return { cohort: 'fresh_install' }
    }
    if (existedBefore === true) {
      const onboarding = storeRef.getOnboarding()
      if (
        onboarding.outcome === 'completed' &&
        onboarding.lastCompletedStep === ONBOARDING_FINAL_STEP
      ) {
        return { cohort: 'upgrade_backfill' }
      }
      return { cohort: 'fresh_install' }
    }
    return { cohort: undefined }
  } catch (err) {
    warnOnce(err instanceof Error ? err.message : String(err))
    return { cohort: undefined }
  }
}

function warnOnce(reason: string): void {
  if (warnedThisSession) {
    return
  }
  warnedThisSession = true
  console.warn('[telemetry-onboarding-cohort] classifier returned undefined', { reason })
}

export function _setStoreForTests(store: Store | null): void {
  storeRef = store
}

export function _resetSessionWarnFlagForTests(): void {
  warnedThisSession = false
}
