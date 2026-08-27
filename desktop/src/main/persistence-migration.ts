import { existsSync, mkdirSync, copyFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import type { OnboardingChecklistState, OnboardingOutcome, OnboardingState } from '../shared/types'
import {
  getDefaultOnboardingState,
  ONBOARDING_FLOW_VERSION,
  ONBOARDING_FINAL_STEP
} from '../shared/constants'
import { MOBILE_PAIRING_USERDATA_FILES } from './runtime/mobile-pairing-files'
import { hardenExistingSecureFile } from '../shared/secure-file'
import { getCanonicalUserDataPath } from './persistence-paths'

/**
 * Copy legacy mobile pairing credentials into the canonical userData directory.
 *
 * Existing installs may already have credentials in the late app.getPath('userData')
 * directory. Before switching the runtime server to the canonical path, copy the
 * registry and E2EE keypair forward as a pair so an update does not force one
 * last re-pair or mix devices with the wrong key.
 */
export function migrateMobilePairingDataToCanonicalUserDataPath(sourceUserDataDir: string): void {
  const targetUserDataDir = getCanonicalUserDataPath()
  if (resolve(sourceUserDataDir) === resolve(targetUserDataDir)) {
    return
  }

  const migrations = MOBILE_PAIRING_USERDATA_FILES.map((fileName) => ({
    sourcePath: join(sourceUserDataDir, fileName),
    targetPath: join(targetUserDataDir, fileName)
  }))
  if (migrations.some(({ sourcePath }) => !existsSync(sourcePath))) {
    return
  }
  if (migrations.some(({ targetPath }) => existsSync(targetPath))) {
    return
  }

  mkdirSync(targetUserDataDir, { recursive: true })
  for (const { sourcePath, targetPath } of migrations) {
    copyFileSync(sourcePath, targetPath)
    // Why: these are credential files (device tokens, E2EE secret key). copyFileSync
    // does not carry Windows ACLs, so re-assert the current-user-only restriction on
    // the copy instead of relying on the runtime's later lazy re-harden on read.
    hardenExistingSecureFile(targetPath)
  }
}

// Why: returns Partial<...> with a partial checklist so the IPC update path
// merges over current state without wiping previously-true keys. Invalid
// top-level fields are OMITTED (not coerced to fallbacks) so partial updates
// don't clobber valid persisted state; the load-path caller spreads defaults.
type SanitizeOnboardingUpdateOptions = {
  migrateLegacyProgress?: boolean
}

function remapLegacyOnboardingLastCompletedStep(
  lastCompletedStep: number,
  raw: Record<string, unknown>
): number {
  if (raw.outcome === 'completed' && lastCompletedStep >= 4) {
    return ONBOARDING_FINAL_STEP
  }
  // Why: v3 was the four-step flow before the Windows terminal preference
  // page. Step 4 already meant notifications, so open progress should resume
  // there rather than treating it as the newly inserted Windows step.
  if (raw.flowVersion === 3) {
    return Math.min(4, lastCompletedStep)
  }
  // Why: v2 was the five-step flow; missing/older versions were seven-step
  // data where step 4 was removed agent setup, not completed integrations.
  if (raw.flowVersion === 2) {
    if (lastCompletedStep === 3) {
      return 2
    }
    if (lastCompletedStep >= 4) {
      return 3
    }
    return lastCompletedStep
  }
  if (lastCompletedStep === 3) {
    return 2
  }
  if (lastCompletedStep === 4) {
    return 2
  }
  if (lastCompletedStep >= 5) {
    return 3
  }
  return lastCompletedStep
}

export function sanitizeOnboardingUpdate(
  input: unknown,
  options: SanitizeOnboardingUpdateOptions = {}
): Partial<Omit<OnboardingState, 'checklist'>> & { checklist?: Partial<OnboardingChecklistState> } {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    return {}
  }
  const raw = input as Record<string, unknown>
  const out: Partial<Omit<OnboardingState, 'checklist'>> & {
    checklist?: Partial<OnboardingChecklistState>
  } = {}

  if ('closedAt' in raw) {
    // Why: `typeof raw.closedAt === 'number'` would let NaN/Infinity through;
    // JSON.stringify writes those as `null` on save, which silently reverts
    // closedAt and re-opens the wizard on next load. Require a finite,
    // non-negative timestamp so live state matches what disk can persist.
    if (typeof raw.closedAt === 'number' && Number.isFinite(raw.closedAt) && raw.closedAt >= 0) {
      out.closedAt = raw.closedAt
    } else if (raw.closedAt === null) {
      out.closedAt = null
    }
    // else: omit — preserve existing persisted value on merge.
  }
  if ('outcome' in raw) {
    const v = raw.outcome
    if (v === 'completed' || v === 'dismissed') {
      out.outcome = v as OnboardingOutcome
    } else if (v === null) {
      out.outcome = null
    }
    // else: omit.
  }
  if ('flowVersion' in raw) {
    const v = raw.flowVersion
    if (typeof v === 'number' && Number.isInteger(v) && v >= 1 && v <= ONBOARDING_FLOW_VERSION) {
      out.flowVersion = v
    }
    // else: omit.
  }
  if ('lastCompletedStep' in raw) {
    const v = raw.lastCompletedStep
    if (typeof v === 'number' && Number.isInteger(v) && v >= -1) {
      const isLegacyFlow =
        options.migrateLegacyProgress && raw.flowVersion !== ONBOARDING_FLOW_VERSION
      // Why: removing two wizard pages changed numeric meanings. Migrate raw
      // legacy disk values before the new final-step bound can drop them.
      const normalized = isLegacyFlow ? remapLegacyOnboardingLastCompletedStep(v, raw) : v
      if (normalized <= ONBOARDING_FINAL_STEP) {
        out.lastCompletedStep = normalized
      }
    }
    // else: omit.
  }
  if ('checklist' in raw) {
    const rawChecklist = raw.checklist
    if (rawChecklist && typeof rawChecklist === 'object' && !Array.isArray(rawChecklist)) {
      // Why: copy ONLY caller-sent boolean keys so partial updates (e.g.
      // `{ addedRepo: true }`) don't reset other checklist items to false.
      const defaults = getDefaultOnboardingState().checklist
      const rc = rawChecklist as Record<string, unknown>
      const checklist: Partial<OnboardingChecklistState> = {}
      for (const key of Object.keys(defaults) as (keyof OnboardingChecklistState)[]) {
        if (key in rc && typeof rc[key] === 'boolean') {
          // Why: perServer is Record<...>, not boolean — skip non-boolean keys
          // so we only copy boolean checklist flags here.
          if (typeof defaults[key] !== 'boolean') {
            continue
          }
          ;(checklist as Record<string, unknown>)[key] = rc[key] as boolean
        }
      }
      out.checklist = checklist
    }
  }
  if (options.migrateLegacyProgress) {
    out.flowVersion = ONBOARDING_FLOW_VERSION
  }
  return out
}
