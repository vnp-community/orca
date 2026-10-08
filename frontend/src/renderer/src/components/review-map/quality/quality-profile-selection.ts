/**
 * quality-profile-selection.ts — FE-CV-TASK-087-04
 *
 * Pure profile-reference helpers. `QualityGate.profile` is "<name>@<scope>/v<version>" while
 * `RunnableProfile.id` is the bare name (contract D5), so refs must be split before comparing.
 *
 * @module components/review-map/quality/quality-profile-selection
 */

import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'

export type ProfileRef = { name: string; scope: string | null; version: number | null }

const PROFILE_REF = /^([^@/]+)(?:@([^/]*)(?:\/v(\d+))?)?$/

/** Never throws: an unrecognised ref is returned whole as the name. */
export function splitProfileRef(ref: string): ProfileRef {
  const match = PROFILE_REF.exec(ref.trim())
  if (!match) {
    return { name: ref.trim(), scope: null, version: null }
  }
  return {
    name: match[1],
    scope: match[2] ? match[2] : null,
    version: match[3] ? Number(match[3]) : null
  }
}

export type ProfileChoice = {
  uiProfile: string | null
  gateProfileRef: string | null
  configuredName: string | null
  runnable: readonly RunnableProfile[]
}

/**
 * Preference: the user's pick, then the profile the gate was computed with, then the
 * configured profile, then the first ready entry. A pick that is no longer runnable is dropped.
 */
export function selectDefaultProfile(choice: ProfileChoice): string | null {
  const ids = new Set(choice.runnable.map((p) => p.id))
  const candidates = [
    choice.uiProfile,
    choice.gateProfileRef ? splitProfileRef(choice.gateProfileRef).name : null,
    choice.configuredName
  ]
  for (const candidate of candidates) {
    if (candidate && ids.has(candidate)) {
      return candidate
    }
  }
  return choice.runnable.find((p) => p.ready)?.id ?? choice.runnable[0]?.id ?? null
}
