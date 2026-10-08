/**
 * C4DiagramCanvas.tsx — FE-CV-TASK-055-03
 *
 * Read-only xyflow canvas over the fixed layer layout. Colors come from CSS variables so the
 * theme applies without hex values.
 */

import { useMemo } from 'react'
import { Background, Controls, ReactFlow, type Edge, type Node, type NodeProps } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Box, Database, TriangleAlert } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { usePrefersReducedMotion } from '../../../hooks/usePrefersReducedMotion'
import { useDocumentColorMode } from '../../../hooks/useDocumentColorMode'
import type { C4Component, C4External } from '../../../../../shared/code-intel-architecture-types'
import { c4OriginLabel } from './C4InferredNotice'
import { C4_NODE_HEIGHT, C4_NODE_WIDTH, type C4Layout } from './c4-layer-layout'
import { c4EdgeStyle } from './c4-edge-style'
import type { C4OverlayFlags } from './c4-overlay-model'
import { relationKey } from './C4RelationsTable'

type ComponentNodeData = { component: C4Component; flags: C4OverlayFlags }
type ExternalNodeData = { external: C4External }
type BandNodeData = { layer: string; width: number; height: number }

function C4ComponentNode({ data, selected }: NodeProps): React.JSX.Element {
  const { component, flags } = data as ComponentNodeData
  return (
    <div
      data-testid={`c4-node-${component.id}`}
      className={cn(
        'rounded-md border bg-card px-2 py-1.5 text-xs text-card-foreground',
        flags.changed && 'border-review-changed border-l-4',
        selected && 'ring-2 ring-ring'
      )}
      style={{ width: C4_NODE_WIDTH, height: C4_NODE_HEIGHT }}
    >
      <div className="flex items-center gap-1 font-medium">
        <Box className="size-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{component.name}</span>
        {flags.violation ? <TriangleAlert className="ml-auto size-3 text-destructive" aria-hidden="true" /> : null}
      </div>
      <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
        {component.techHint ? `${component.techHint} · ` : ''}
        {translate('auto.components.reviewMap.c4.symbols', '{{count}} symbols', { count: component.symbolCount })}
      </div>
      <div className="mt-1 flex flex-wrap gap-1 text-[10px]">
        <Badge variant="outline">{c4OriginLabel(component.origin)}</Badge>
        {flags.changed ? <span>{translate('auto.components.reviewMap.c4.flag.changed', 'changed')}</span> : null}
        {flags.untested ? <span>{translate('auto.components.reviewMap.c4.flag.untested', 'untested')}</span> : null}
      </div>
    </div>
  )
}

function C4ExternalNode({ data, selected }: NodeProps): React.JSX.Element {
  const { external } = data as ExternalNodeData
  return (
    <div
      data-testid={`c4-node-${external.id}`}
      className={cn('rounded-md border border-dashed bg-muted/40 px-2 py-1.5 text-xs', selected && 'ring-2 ring-ring')}
      style={{ width: C4_NODE_WIDTH, height: C4_NODE_HEIGHT }}
    >
      <div className="flex items-center gap-1 font-medium">
        <Database className="size-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{external.name}</span>
      </div>
      <div className="mt-0.5 text-[11px] text-muted-foreground">{external.kind}</div>
    </div>
  )
}

function C4LayerBand({ data }: NodeProps): React.JSX.Element {
  const { layer, width, height } = data as BandNodeData
  return (
    <div className="rounded-lg bg-muted/30" style={{ width, height }} aria-hidden="true">
      <span className="p-1 text-[10px] uppercase tracking-wide text-muted-foreground">
        {translate(`auto.components.reviewMap.c4.layer.${layer}`, layer)}
      </span>
    </div>
  )
}

const NODE_TYPES = { c4Component: C4ComponentNode, c4External: C4ExternalNode, c4Band: C4LayerBand }

export function C4DiagramCanvas({
  layout,
  flagsById,
  changedEdgeKeys,
  selectedId,
  selectedEdgeKey,
  onSelectNode,
  onSelectEdge
}: {
  layout: C4Layout
  flagsById: ReadonlyMap<string, C4OverlayFlags>
  changedEdgeKeys: ReadonlySet<string>
  selectedId: string | null
  selectedEdgeKey: string | null
  onSelectNode: (id: string) => void
  onSelectEdge: (key: string) => void
}): React.JSX.Element {
  const colorMode = useDocumentColorMode()
  const reduceMotion = usePrefersReducedMotion()

  const nodes = useMemo<Node[]>(() => {
    const bands: Node[] = layout.bands.map((b) => ({
      id: b.id,
      type: 'c4Band',
      position: { x: b.x, y: b.y },
      data: { layer: b.layer, width: b.width, height: b.height },
      selectable: false,
      draggable: false,
      zIndex: -1
    }))
    const real: Node[] = layout.nodes.map((n) => ({
      id: n.id,
      type: n.type === 'component' ? 'c4Component' : 'c4External',
      position: { x: n.x, y: n.y },
      selected: n.id === selectedId,
      data:
        n.type === 'component'
          ? { component: n.component, flags: flagsById.get(n.id) ?? { changed: false, untested: false, violation: false, affected: false } }
          : { external: n.external }
    }))
    return [...bands, ...real]
  }, [layout, flagsById, selectedId])

  const edges = useMemo<Edge[]>(
    () =>
      layout.relations.map((r) => {
        const style = c4EdgeStyle(r)
        const key = relationKey(r)
        const stroke = style.violation
          ? 'var(--destructive)'
          : changedEdgeKeys.has(key)
            ? 'var(--review-changed)'
            : 'var(--muted-foreground)'
        return {
          id: key,
          source: r.from,
          target: r.to,
          selected: key === selectedEdgeKey,
          label: r.label,
          style: {
            stroke,
            strokeWidth: style.strokeWidth,
            strokeDasharray: style.strokeDasharray,
            opacity: style.opacity
          }
        }
      }),
    [layout, changedEdgeKeys, selectedEdgeKey]
  )

  return (
    <div className="min-h-0 flex-1" role="img" aria-label={translate('auto.components.reviewMap.c4.diagramLabel', 'Component diagram; use the list view for a text equivalent')}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        colorMode={colorMode}
        nodesDraggable={false}
        nodesConnectable={false}
        onlyRenderVisibleElements
        fitView
        fitViewOptions={{ padding: 0.15, duration: reduceMotion ? 0 : 200 }}
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_e, node) => {
          if (node.type !== 'c4Band') {
            onSelectNode(node.id)
          }
        }}
        onEdgeClick={(_e, edge) => onSelectEdge(edge.id)}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}
