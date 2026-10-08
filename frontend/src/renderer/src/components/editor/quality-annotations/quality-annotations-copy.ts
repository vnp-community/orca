/**
 * quality-annotations-copy.ts — FE-CV-TASK-087-10
 *
 * English table of the diff-annotation strings (catalog group `annotations`). Call `qa(key)` at
 * render time, never at module scope.
 *
 * @module components/editor/quality-annotations/quality-annotations-copy
 */

import { createQualityCopy } from '../../review-map/quality/quality-copy-factory'

export const QUALITY_ANNOTATIONS_COPY = {
  toggleLabel: 'Check annotations',
  toggleOn: 'Hide check annotations',
  toggleOff: 'Show check annotations',
  noticeContentChanged:
    'Check annotations are hidden because the content changed. Re-run checks to refresh them.',
  noticeDirty:
    'Check annotations may be a few lines off: the run started with uncommitted changes.',
  noticeChangedDuringRun:
    'Check annotations may be off: files changed while the checks were running.',
  glyphCount: '{{count}} check findings on this line',
  glyphMore: '+{{count}} more'
} as const

export const qa = createQualityCopy('annotations', QUALITY_ANNOTATIONS_COPY)
