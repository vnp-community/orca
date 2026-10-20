/**
 * ReviewArtifactRevisions — FE-OR-T212
 *
 * The revisions of one drawing, newest first. A person can compare any of them with the one in force, and put an older one back
 * (which costs nothing: it calls no agent).
 *
 * @module components/request/outcome-review/artifact/ReviewArtifactRevisions
 */

import React, { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { translate } from '@/i18n/i18n'
import { fetchRevisions, restoreRevision } from '../../../../hooks/reviewArtifactCalls'
import { ReviewArtifactCompareDialog } from './ReviewArtifactCompareDialog'
import { actionText, originText } from './review-artifact-copy'
import { errorText } from '../outcome-review-state-copy'
import type { DiagramRef, ReviewArtifactMeta } from '../../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.artifact.revisions.'

type Props = {
  requestId: string
  current: ReviewArtifactMeta
  diagram: DiagramRef
  canRestore: boolean
  onChanged?: () => void
}

export function ReviewArtifactRevisions({
  requestId,
  current,
  diagram,
  canRestore,
  onChanged
}: Props): React.JSX.Element {
  const [list, setList] = useState<ReviewArtifactMeta[] | null>(null)
  const [failure, setFailure] = useState<string | null>(null)
  const [compareWith, setCompareWith] = useState<ReviewArtifactMeta | null>(null)

  const load = useCallback(async () => {
    setFailure(null)
    const res = await fetchRevisions(requestId, {
      section: diagram.section,
      variant: diagram.variant,
      name: diagram.name
    })
    if (res.ok) {
      setList(res.value)
    } else {
      setFailure(errorText(res.error.code))
    }
  }, [requestId, diagram])

  const restore = async (rev: ReviewArtifactMeta): Promise<void> => {
    const res = await restoreRevision(requestId, rev.artifactId, '')
    if (!res.ok) {
      toast.error(errorText(res.error.code))
      return
    }
    toast.success(
      translate(`${T}restored`, 'Revision {{revision}} is in force again', {
        revision: rev.revision
      })
    )
    setList(null)
    onChanged?.()
  }

  return (
    <>
      <Popover onOpenChange={(open) => open && void load()}>
        <PopoverTrigger asChild>
          <Button size="xs" variant="ghost">
            {actionText('revisions')}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-96" align="end" data-testid="review-artifact-revisions">
          {failure && (
            <p role="alert" className="text-sm text-destructive">
              {failure}
            </p>
          )}
          {!failure && list === null && (
            <p className="text-sm text-muted-foreground">
              {translate(`${T}loading`, 'Loading revisions…')}
            </p>
          )}
          {list !== null && (
            <ul className="flex flex-col gap-2">
              {list.map((rev) => {
                const inForce = rev.artifactId === current.artifactId
                return (
                  <li
                    key={rev.artifactId}
                    className="flex items-center gap-2 text-sm"
                    data-testid="review-artifact-revision"
                  >
                    <span className="min-w-0 flex-1">
                      {translate(`${T}row`, 'Revision {{revision}} · {{origin}}', {
                        revision: rev.revision,
                        origin: originText(rev.origin)
                      })}
                      {inForce && (
                        <span className="ml-1 text-xs text-muted-foreground">
                          {translate(`${T}inForce`, '(in force)')}
                        </span>
                      )}
                    </span>
                    {!inForce && rev.state !== 'failed' && (
                      <>
                        <Button size="xs" variant="ghost" onClick={() => setCompareWith(rev)}>
                          {actionText('compare')}
                        </Button>
                        {canRestore && (
                          <Button size="xs" variant="outline" onClick={() => void restore(rev)}>
                            {actionText('restore')}
                          </Button>
                        )}
                      </>
                    )}
                  </li>
                )
              })}
            </ul>
          )}
        </PopoverContent>
      </Popover>
      {compareWith && (
        <ReviewArtifactCompareDialog
          requestId={requestId}
          from={compareWith}
          to={current}
          diagram={diagram}
          onClose={() => setCompareWith(null)}
        />
      )}
    </>
  )
}
