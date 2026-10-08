/**
 * QualityLensBlockHost.tsx — FE-CV-TASK-087-07
 *
 * One collapsible block of the quality lens. The block module is imported, and so mounted,
 * only after the user opens it; the open state is kept in the quality UI state.
 *
 * @module components/review-map/quality/QualityLensBlockHost
 */

import { Suspense, lazy, useMemo } from 'react'
import { ChevronRight } from 'lucide-react'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import type { QualityLensBlock } from './quality-lens-blocks'
import type { QualityBlockProps } from './quality-lens-block-types'
import { scorecardCopy } from './quality-scorecard-copy'

export function QualityLensBlockHost({
  block,
  open,
  onOpenChange,
  blockProps
}: {
  block: QualityLensBlock
  open: boolean
  onOpenChange: (open: boolean) => void
  blockProps: QualityBlockProps
}): React.JSX.Element {
  const Content = useMemo(() => lazy(block.load), [block])
  return (
    <Collapsible open={open} onOpenChange={onOpenChange} data-testid={`quality-block-${block.id}`}>
      <CollapsibleTrigger className="flex w-full items-center gap-1 py-1 text-sm font-medium text-foreground">
        <ChevronRight
          className={`size-3.5 transition-transform ${open ? 'rotate-90' : ''}`}
          aria-hidden
        />
        {scorecardCopy(block.titleKey)}
      </CollapsibleTrigger>
      <CollapsibleContent>
        {open ? (
          <Suspense
            fallback={
              <Skeleton
                className="h-24 w-full animate-none"
                aria-label={scorecardCopy('blocks.loading')}
              />
            }
          >
            <Content {...blockProps} />
          </Suspense>
        ) : null}
      </CollapsibleContent>
    </Collapsible>
  )
}
