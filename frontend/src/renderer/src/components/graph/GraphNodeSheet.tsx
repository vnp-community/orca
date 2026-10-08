/**
 * GraphNodeSheet — FE-REQ-TASK-032-06
 *
 * Plain-text node details. No HTML rendering and no way to edit risk.
 *
 * @module components/graph/GraphNodeSheet
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { RiskBadge } from './RiskBadge'
import type { GraphNode, GraphPayload } from '../../../../shared/graph-types'

type Props = {
  payload: GraphPayload
  nodeId: string | null
  onClose: () => void
  onOpenNode?: (node: GraphNode) => void
}

export function GraphNodeSheet({ payload, nodeId, onClose, onOpenNode }: Props): React.JSX.Element {
  const node = nodeId ? payload.nodes.find((n) => n.id === nodeId) : undefined
  const label = (id: string): string => payload.nodes.find((n) => n.id === id)?.label ?? id
  const incoming = node ? payload.edges.filter((e) => e.to === node.id) : []
  const outgoing = node ? payload.edges.filter((e) => e.from === node.id) : []

  return (
    <Sheet open={node !== undefined} onOpenChange={(open) => { if (!open) {onClose()} }}>
      <SheetContent side="right" className="gap-3 p-4">
        {node ? (
          <>
            <SheetHeader className="p-0">
              <SheetTitle className="break-words">{node.label}</SheetTitle>
              <SheetDescription>
                {node.kind}
                {node.group ? ` · ${node.group}` : ''}
              </SheetDescription>
            </SheetHeader>
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <RiskBadge level={node.risk} assessedAt={payload.assessedAt} tool={payload.tool} />
              {node.status ? <span className="text-muted-foreground">{node.status}</span> : null}
            </div>
            <section aria-label={translate('auto.components.graph.Sheet.incoming', 'Incoming')}>
              <h3 className="text-xs font-medium">{translate('auto.components.graph.Sheet.incoming', 'Incoming')} ({incoming.length})</h3>
              <ul className="mt-1 space-y-0.5 text-xs text-muted-foreground">
                {incoming.map((e, i) => <li key={i}>{label(e.from)} — {e.kind}</li>)}
              </ul>
            </section>
            <section aria-label={translate('auto.components.graph.Sheet.outgoing', 'Outgoing')}>
              <h3 className="text-xs font-medium">{translate('auto.components.graph.Sheet.outgoing', 'Outgoing')} ({outgoing.length})</h3>
              <ul className="mt-1 space-y-0.5 text-xs text-muted-foreground">
                {outgoing.map((e, i) => <li key={i}>{label(e.to)} — {e.kind}</li>)}
              </ul>
            </section>
            {node.meta?.findingIds?.length ? (
              <section>
                <h3 className="text-xs font-medium">{translate('auto.components.graph.Sheet.findings', 'Findings')}</h3>
                <ul className="mt-1 text-xs text-muted-foreground">
                  {node.meta.findingIds.map((id) => <li key={id}>{id}</li>)}
                </ul>
              </section>
            ) : null}
            {onOpenNode ? (
              <Button size="sm" variant="outline" className="self-start" onClick={() => onOpenNode(node)}>
                {translate('auto.components.graph.Sheet.open', 'Open')}
              </Button>
            ) : null}
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  )
}
