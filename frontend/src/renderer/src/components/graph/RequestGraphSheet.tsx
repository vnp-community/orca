/**
 * RequestGraphSheet — FE-REQ-TASK-032-08
 *
 * Right-hand sheet hosting GraphPanel; the panel (and xyflow behind it) loads
 * lazily so it never lands in the Requests page chunk.
 *
 * @module components/graph/RequestGraphSheet
 */

import React, { Suspense } from 'react'
import { translate } from '@/i18n/i18n'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { GraphSkeleton } from './GraphStates'
import type { GraphPanelProps } from './GraphPanel'

const GraphPanel = React.lazy(() => import('./GraphPanel').then((m) => ({ default: m.GraphPanel })))

type Props = Pick<GraphPanelProps, 'request' | 'subject' | 'lensInitial' | 'impactAssessed' | 'onNodeOpen'> & {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function RequestGraphSheet({ open, onOpenChange, ...panel }: Props): React.JSX.Element {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full gap-0 p-0 sm:max-w-4xl" data-testid="request-graph-sheet">
        <SheetHeader className="border-b border-border px-4 py-3">
          <SheetTitle>{translate('auto.components.graph.Entry.title', 'Request graph')}</SheetTitle>
          <SheetDescription className="sr-only">
            {translate('auto.components.graph.Entry.description', 'Explore the request flow, plan and impact as a graph or list')}
          </SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1">
          {open ? (
            <Suspense fallback={<GraphSkeleton />}>
              <GraphPanel {...panel} />
            </Suspense>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  )
}
