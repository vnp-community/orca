/**
 * ReviewArtifactInfoBar — FE-OR-T212
 *
 * Under a drawing: that an agent drew it and it may be wrong, which skill and revision it is, when and how it was made, and the
 * ways to act on it: open it larger, regenerate it, see its revisions, compare two of them.
 *
 * @module components/request/outcome-review/artifact/ReviewArtifactInfoBar
 */

import React from 'react'
import { Button } from '@/components/ui/button'
import { ReviewArtifactRevisions } from './ReviewArtifactRevisions'
import { actionText, drawnByAgentText, infoText } from './review-artifact-copy'
import type { RegeneratePreset } from './ReviewArtifactView'
import type { DiagramRef, ReviewArtifactMeta } from '../../../../../../shared/outcome-review-types'

type Props = {
  requestId: string
  meta: ReviewArtifactMeta
  diagram: DiagramRef
  canRegenerate: boolean
  onOpenLarger: () => void
  onRegenerate?: (preset: RegeneratePreset) => void
  onChanged?: () => void
}

function whenText(iso: string): string {
  const time = Date.parse(iso)
  return Number.isNaN(time) ? '' : new Date(time).toLocaleString()
}

export function ReviewArtifactInfoBar({
  requestId,
  meta,
  diagram,
  canRegenerate,
  onOpenLarger,
  onRegenerate,
  onChanged
}: Props): React.JSX.Element {
  return (
    <figcaption
      className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground"
      data-testid="review-artifact-info"
    >
      <span>{drawnByAgentText(meta.skillVersion.startsWith('orca-builtin'))}</span>
      <span data-testid="review-artifact-meta">
        {infoText(meta, whenText(meta.createdAt), meta.origin)}
      </span>
      <span className="ml-auto flex items-center gap-1">
        <Button size="xs" variant="ghost" onClick={onOpenLarger}>
          {actionText('openLarger')}
        </Button>
        <ReviewArtifactRevisions
          requestId={requestId}
          current={meta}
          diagram={diagram}
          canRestore={canRegenerate}
          onChanged={onChanged}
        />
        {canRegenerate && onRegenerate && (
          <Button
            size="xs"
            variant="outline"
            onClick={() => onRegenerate({ sections: [diagram.section] })}
          >
            {actionText('regenerate')}
          </Button>
        )}
      </span>
    </figcaption>
  )
}
