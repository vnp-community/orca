/**
 * notes-i18n.ts — FE-CV-TASK-060-03
 *
 * Translation helper for review notes, sent batches and turn comparison.
 * Called at render time only (never at module level).
 *
 * @module components/review-map/notes/notes-i18n
 */

import { translate } from '@/i18n/i18n'

export const NOTES_I18N_BASE = 'auto.components.reviewMap'

/** `key` is the full suffix after `auto.components.reviewMap.`, e.g. `ReviewNote.save`. */
export function tn(key: string, fallback: string, params?: Record<string, unknown>): string {
  return translate(`${NOTES_I18N_BASE}.${key}`, fallback, params)
}
