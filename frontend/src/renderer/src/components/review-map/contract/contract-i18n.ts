/**
 * contract-i18n.ts — FE-CV-TASK-059-04
 *
 * Translation helper for the Contract lens. Called at render time only (never at module level).
 *
 * @module components/review-map/contract/contract-i18n
 */

import { translate } from '@/i18n/i18n'

export const CONTRACT_I18N_BASE = 'auto.components.reviewMap.contract'

export function tc(key: string, fallback: string, params?: Record<string, unknown>): string {
  return translate(`${CONTRACT_I18N_BASE}.${key}`, fallback, params)
}
