/**
 * ReviewArtifactFrame — FE-OR-T211, T212
 *
 * One drawing of a review with what goes with it: the line that says an agent drew it, its revision, and the ways to enlarge,
 * regenerate, revisit and compare it. The drawing is read when the frame is shown, and a drawing that was read before comes
 * from memory (see useReviewArtifact).
 *
 * @module components/request/outcome-review/artifact/ReviewArtifactFrame
 */

import React, { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { useReviewArtifact } from '../../../../hooks/useReviewArtifact'
import { ReviewArtifactInfoBar } from './ReviewArtifactInfoBar'
import { ReviewArtifactView, type RegeneratePreset } from './ReviewArtifactView'
import { actionText } from './review-artifact-copy'
import type { DiagramRef } from '../../../../../../shared/outcome-review-types'

export type { RegeneratePreset } from './ReviewArtifactView'

type Props = {
  requestId: string
  diagram: DiagramRef
  label: string
  canRegenerate: boolean
  onRegenerate?: (preset: RegeneratePreset) => void
  /** A revision was put back in force, or the drawing is gone: the review should be read again. */
  onChanged?: () => void
}

const ZOOMS = [0.5, 0.75, 1, 1.5, 2]
const MAX_ZOOM = ZOOMS.at(-1) ?? 2

export function ReviewArtifactFrame({
  requestId,
  diagram,
  label,
  canRegenerate,
  onRegenerate,
  onChanged
}: Props): React.JSX.Element {
  const drawn = diagram.status === 'cached' && !!diagram.artifactId
  const artifact = useReviewArtifact(requestId, diagram.artifactId, drawn)
  const [large, setLarge] = useState(false)
  const [zoom, setZoom] = useState(1)
  const view = { artifact, diagram, label, canRegenerate, onRegenerate, onChanged }
  const step = (delta: number): void =>
    setZoom(ZOOMS[Math.min(ZOOMS.length - 1, Math.max(0, ZOOMS.indexOf(zoom) + delta))] ?? 1)
  return (
    <figure
      className="flex flex-col gap-1"
      data-testid="review-artifact"
      data-artifact-id={diagram.artifactId}
    >
      <ReviewArtifactView {...view} />
      {drawn && artifact.status === 'ready' && artifact.meta && (
        <ReviewArtifactInfoBar
          requestId={requestId}
          meta={artifact.meta}
          diagram={diagram}
          canRegenerate={canRegenerate}
          onOpenLarger={() => setLarge(true)}
          onRegenerate={onRegenerate}
          onChanged={onChanged}
        />
      )}
      {large && (
        <Dialog open onOpenChange={(open) => !open && setLarge(false)}>
          <DialogContent
            className="flex h-[90vh] max-w-[95vw] flex-col sm:max-w-[95vw]"
            data-testid="review-artifact-large"
          >
            <DialogHeader>
              <DialogTitle>{artifact.meta?.title || label}</DialogTitle>
              <DialogDescription className="sr-only">{label}</DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2">
              <Button
                size="xs"
                variant="outline"
                disabled={zoom <= ZOOMS[0]}
                onClick={() => step(-1)}
              >
                {actionText('zoomOut')}
              </Button>
              <Button
                size="xs"
                variant="outline"
                disabled={zoom >= MAX_ZOOM}
                onClick={() => step(1)}
              >
                {actionText('zoomIn')}
              </Button>
              <span className="text-xs text-muted-foreground tabular-nums">
                {Math.round(zoom * 100)}%
              </span>
            </div>
            {/* The zoom is a CSS transform on a wrapper outside the frame: nothing inside the drawing runs. */}
            <div className="min-h-0 flex-1 overflow-auto">
              <div
                style={{
                  transform: `scale(${zoom})`,
                  transformOrigin: 'top left',
                  width: `${100 / zoom}%`
                }}
              >
                <ReviewArtifactView {...view} uncapped />
              </div>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </figure>
  )
}
