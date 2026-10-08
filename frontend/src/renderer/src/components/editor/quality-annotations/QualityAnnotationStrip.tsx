/**
 * QualityAnnotationStrip.tsx — FE-CV-TASK-087-11
 *
 * Compact row under a diff: why annotations are hidden or approximate, plus the on/off switch
 * (`ui.annotationsOn`). Renders nothing when quality is off, so non-quality users see no change.
 *
 * @module components/editor/quality-annotations/QualityAnnotationStrip
 */

import React from 'react'
import { useAppStore } from '@/store'
import { Button } from '../../ui/button'
import { DEFAULT_QUALITY_UI } from '../../../store/slices/code-intel-quality-state-types'
import { qualityAnnotationNoticeText } from './quality-annotation-notice'
import type { QualityAnnotationNoticeKind } from './quality-annotation-notice'
import { qa } from './quality-annotations-copy'

export type QualityAnnotationStripProps = {
  worktreeId: string | null | undefined
  notice: QualityAnnotationNoticeKind | null
  toggleVisible: boolean
}

export function QualityAnnotationStrip({
  worktreeId,
  notice,
  toggleVisible
}: QualityAnnotationStripProps): React.JSX.Element | null {
  const annotationsOn = useAppStore((s) =>
    worktreeId
      ? (s.codeIntelQualityByWorktree[worktreeId]?.ui.annotationsOn ??
        DEFAULT_QUALITY_UI.annotationsOn)
      : false
  )
  if (!worktreeId || (!notice && !toggleVisible)) {
    return null
  }
  return (
    <div
      className="flex items-center justify-between gap-2 border-b border-border/60 bg-muted/40 px-3 py-1 text-xs text-muted-foreground"
      data-testid="quality-annotation-strip"
    >
      <span className="min-w-0 truncate" role="status">
        {notice ? qualityAnnotationNoticeText(notice) : null}
      </span>
      {toggleVisible ? (
        <Button
          type="button"
          variant="ghost"
          size="xs"
          aria-pressed={annotationsOn}
          title={annotationsOn ? qa('toggleOn') : qa('toggleOff')}
          onClick={() =>
            useAppStore.getState().setQualityUi(worktreeId, { annotationsOn: !annotationsOn })
          }
        >
          {qa('toggleLabel')}
        </Button>
      ) : null}
    </div>
  )
}
