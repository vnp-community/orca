/**
 * findings-i18n.ts — FE-CV-TASK-059-05
 *
 * Translation helper for the structural Findings panel. Called at render time only.
 *
 * @module components/review-map/findings/findings-i18n
 */

import { translate } from '@/i18n/i18n'
import { FINDING_BASE } from './finding-view-model'

export function tf(key: string, fallback: string, params?: Record<string, unknown>): string {
  return translate(`${FINDING_BASE}.${key}`, fallback, params)
}
