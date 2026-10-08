/**
 * ErdCanvas.tsx — FE-CV-TASK-057-05
 *
 * Read-only xyflow canvas over the deterministic layout. Colours come from CSS variables.
 */

import { useEffect, useMemo } from 'react'
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Edge,
  type Node
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { translate } from '@/i18n/i18n'
import { useDocumentColorMode } from '../../../hooks/useDocumentColorMode'
import { usePrefersReducedMotion } from '../../../hooks/usePrefersReducedMotion'
import { ErdGhostTableNode, type ErdGhostNodeData } from './ErdGhostTableNode'
import { ErdSchemaGroupNode } from './ErdSchemaGroupNode'
import { ErdTableNode, type ErdTableNodeData } from './ErdTableNode'
import { erdEdgeFor } from './ErdRelationEdge'
import { ERD_NODE_WIDTH } from './erd-layout'
import type { ErdLayoutResult } from './erd-layout'
import type { ErdGhostRef, ErdRelationView, ErdTableView } from './erd-view-model'

// Declared once: xyflow remounts every node when this object identity changes.
const NODE_TYPES = {
  erdTable: ErdTableNode,
  erdGhost: ErdGhostTableNode,
  erdGroup: ErdSchemaGroupNode
}
const MINIMAP_MAX_NODES = 80
const GHOST_GAP = 200
const GHOST_ROW = 72

export type ErdCanvasProps = {
  service: string
  tables: readonly ErdTableView[]
  relations: readonly ErdRelationView[]
  ghosts: readonly ErdGhostRef[]
  layout: ErdLayoutResult
  expandedTables: ReadonlySet<string>
  query: string
  dimmed: ReadonlySet<string>
  selectedKey: string | null
  onSelect: (key: string) => void
  onToggleExpand: (key: string) => void
  onOpenService: (service: string) => void
}

export const ghostNodeId = (g: { service: string; table: string }): string =>
  `ghost:${g.service}.${g.table}`

function CanvasInner(props: ErdCanvasProps): React.JSX.Element {
  const { service, tables, relations, ghosts, layout, expandedTables, query, dimmed, selectedKey } =
    props
  const colorMode = useDocumentColorMode()
  const reduceMotion = usePrefersReducedMotion()
  const flow = useReactFlow()

  const { nodes, edges } = useMemo(() => {
    const nodes: Node[] = layout.groups.map((g) => ({
      id: `group:${g.schema}`,
      type: 'erdGroup',
      position: { x: g.x, y: g.y },
      selectable: false,
      draggable: false,
      zIndex: -1,
      data: { schema: g.schema, width: g.width, height: g.height }
    }))
    let maxX = 0
    for (const t of tables) {
      const r = layout.positions.get(t.key)
      if (!r) {
        continue
      }
      maxX = Math.max(maxX, r.x + r.width)
      const data: ErdTableNodeData = {
        table: t,
        expanded: expandedTables.has(t.key),
        query,
        dimmed: dimmed.has(t.key),
        onSelect: props.onSelect,
        onToggleExpand: props.onToggleExpand
      }
      nodes.push({
        id: t.key,
        type: 'erdTable',
        position: { x: r.x, y: r.y },
        selected: t.key === selectedKey,
        data
      })
    }
    const byName = new Map(tables.map((t) => [t.name, t.key]))
    const resolve = (e: { service?: string; table: string }): string | null => {
      if (e.service && e.service !== service) {
        return ghostNodeId({ service: e.service, table: e.table })
      }
      return byName.get(e.table) ?? null
    }
    const used = new Set<string>()
    const edges: Edge[] = []
    for (const rel of relations) {
      const from = resolve(rel.from)
      const to = resolve(rel.to)
      if (!from || !to) {
        continue
      }
      used.add(from)
      used.add(to)
      edges.push(erdEdgeFor(rel, from, to, false))
    }
    ghosts
      .filter((g) => used.has(ghostNodeId(g)))
      .forEach((g, i) => {
        const data: ErdGhostNodeData = { ...g, onOpenService: props.onOpenService }
        nodes.push({
          id: ghostNodeId(g),
          type: 'erdGhost',
          position: { x: maxX + GHOST_GAP, y: i * GHOST_ROW },
          data
        })
      })
    return { nodes, edges }
    // Callbacks are stable props from the lens; the rest drives layout.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tables, relations, ghosts, layout, expandedTables, query, dimmed, selectedKey, service])

  useEffect(() => {
    if (!selectedKey) {
      return
    }
    const r = layout.positions.get(selectedKey)
    if (r) {
      void flow.setCenter(r.x + ERD_NODE_WIDTH / 2, r.y + r.height / 2, {
        zoom: flow.getZoom(),
        duration: reduceMotion ? 0 : 200
      })
    }
    // Re-center only when the selection itself changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedKey])

  return (
    <div
      className="min-h-0 flex-1"
      role="img"
      aria-label={translate(
        'auto.components.reviewMap.ErdCanvas.label',
        'Entity-relationship diagram; use the list view for a text equivalent'
      )}
    >
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
          if (node.type === 'erdTable') {
            props.onSelect(node.id)
          }
        }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        {nodes.length <= MINIMAP_MAX_NODES ? <MiniMap pannable zoomable /> : null}
      </ReactFlow>
    </div>
  )
}

export function ErdCanvas(props: ErdCanvasProps): React.JSX.Element {
  return (
    <ReactFlowProvider>
      <CanvasInner {...props} />
    </ReactFlowProvider>
  )
}
