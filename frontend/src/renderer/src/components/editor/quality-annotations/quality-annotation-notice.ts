/**
 * quality-annotation-notice.ts — FE-CV-TASK-087-10
 *
 * The compact notice shown under the diff header when annotations are hidden or may be off.
 * Never a toast: the cause is local to this diff.
 *
 * @module components/editor/quality-annotations/quality-annotation-notice
 */

import type { QualityAnnotationSoftWarning } from './quality-annotation-eligibility'
import { qa } from './quality-annotations-copy'

export type QualityAnnotationNoticeKind = 'content-changed' | QualityAnnotationSoftWarning

export function resolveQualityAnnotationNotice(args: {
  contentChanged: boolean
  softWarning?: QualityAnnotationSoftWarning
}): QualityAnnotationNoticeKind | null {
  if (args.contentChanged) {
    return 'content-changed'
  }
  return args.softWarning ?? null
}

export function qualityAnnotationNoticeText(kind: QualityAnnotationNoticeKind): string {
  switch (kind) {
    case 'content-changed':
      return qa('noticeContentChanged')
    case 'changed-during-run':
      return qa('noticeChangedDuringRun')
    default:
      return qa('noticeDirty')
  }
}
