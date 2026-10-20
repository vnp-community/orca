/**
 * ReviewArtifactCompareDialog — FE-OR-T212
 *
 * Two revisions of a drawing side by side, with the boxes and arrows that differ between the models they were drawn from. The
 * difference is computed by the server from the stored models; no agent is asked.
 *
 * @module components/request/outcome-review/artifact/ReviewArtifactCompareDialog
 */

import React, { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { compareRevisions } from '../../../../hooks/reviewArtifactCalls'
import { useReviewArtifact } from '../../../../hooks/useReviewArtifact'
import { ReviewArtifactView } from './ReviewArtifactView'
import {
  actionText,
  compareTitleText,
  modelChangeText,
  noModelChangeText,
  type ModelChangeId
} from './review-artifact-copy'
import { errorText } from '../outcome-review-state-copy'
import type {
  DiagramRef,
  ModelDiff,
  ReviewArtifactMeta
} from '../../../../../../shared/outcome-review-types'

const CHANGES: ModelChangeId[] = [
  'addedNodes',
  'removedNodes',
  'changedNodes',
  'addedEdges',
  'removedEdges',
  'changedEdges'
]

type Props = {
  requestId: string
  from: ReviewArtifactMeta
  to: ReviewArtifactMeta
  diagram: DiagramRef
  onClose: () => void
}

function Side({
  requestId,
  meta,
  diagram
}: {
  requestId: string
  meta: ReviewArtifactMeta
  diagram: DiagramRef
}): React.JSX.Element {
  const artifact = useReviewArtifact(requestId, meta.artifactId)
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-xs font-medium">r{meta.revision}</span>
      <ReviewArtifactView
        artifact={artifact}
        diagram={{ ...diagram, artifactId: meta.artifactId, status: 'cached' }}
        label={`r${meta.revision}`}
        canRegenerate={false}
        uncapped
      />
    </div>
  )
}

export function ReviewArtifactCompareDialog({
  requestId,
  from,
  to,
  diagram,
  onClose
}: Props): React.JSX.Element {
  const [diff, setDiff] = useState<ModelDiff | null>(null)
  const [failure, setFailure] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    void compareRevisions(requestId, from.artifactId, to.artifactId).then((res) => {
      if (!live) {
        return
      }
      if (res.ok) {
        setDiff(res.value)
      } else {
        setFailure(errorText(res.error.code))
      }
    })
    return () => {
      live = false
    }
  }, [requestId, from.artifactId, to.artifactId])

  const changes = diff ? CHANGES.filter((id) => diff[id].length > 0) : []
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        className="max-h-[90vh] max-w-[95vw] overflow-y-auto sm:max-w-5xl"
        data-testid="review-artifact-compare"
      >
        <DialogHeader>
          <DialogTitle>{compareTitleText()}</DialogTitle>
          <DialogDescription className="sr-only">{compareTitleText()}</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          {[from, to].map((meta) => (
            <Side key={meta.artifactId} requestId={requestId} meta={meta} diagram={diagram} />
          ))}
        </div>
        {failure && (
          <p role="alert" className="text-sm text-destructive">
            {failure}
          </p>
        )}
        {diff && changes.length === 0 && (
          <p className="text-sm text-muted-foreground">{noModelChangeText()}</p>
        )}
        {changes.length > 0 && (
          <dl
            className="grid grid-cols-1 gap-2 text-sm md:grid-cols-2"
            data-testid="review-artifact-model-diff"
          >
            {changes.map((id) => (
              <div key={id}>
                <dt className="font-medium">{modelChangeText(id)}</dt>
                <dd className="text-muted-foreground">{diff![id].join(', ')}</dd>
              </div>
            ))}
          </dl>
        )}
        <Button variant="outline" size="sm" className="justify-self-end" onClick={onClose}>
          {actionText('close')}
        </Button>
      </DialogContent>
    </Dialog>
  )
}
