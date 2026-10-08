/**
 * GraphCanvas — FE-REQ-TASK-032-05
 *
 * One xyflow canvas for every lens. Lenses are data; layout is pluggable.
 * Loaded lazily by GraphPanel so xyflow stays out of the main chunk.
 *
 * @module components/graph/GraphCanvas
 */

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Background, Controls, Handle, MiniMap, Position, ReactFlow,
  type Edge, type Node, type NodeProps, type ReactFlowInstance
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { useDocumentColorMode } from '../../hooks/useDocumentColorMode'
import { usePrefersReducedMotion } from '../../hooks/usePrefersReducedMotion'
import { GraphEdgeLine } from './GraphEdgeLine'
import { GraphGroupNode } from './GraphGroupNode'
import { GraphNodeCard, type GraphNavigateDirection } from './GraphNodeCard'
import { applyChangeView, type GraphChangeView } from './graph-before-after'
import { focusNeighborhood, isDimmed } from './graph-focus-state'
import { GROUP_NODE_PREFIX, buildGroupEdges, pickVisibleNodes, type GraphGroup } from './graph-grouping'
import { layoutCacheKey, waveLayoutEngine, type LayoutEngine, type LayoutPositions } from './graph-layout-engine'
import { levelForZoom, zoomPresentation, type GraphZoomLevel } from './graph-zoom-levels'
import type { GraphNode, GraphPayload } from '../../../../shared/graph-types'

const ZOOM_DEBOUNCE_MS = 100
const LAYOUT_CACHE_MAX = 5
const MINIMAP_NODE_THRESHOLD = 30
const ONLY_VISIBLE_THRESHOLD = 200

type NodeData = {
  node?: GraphNode
  group?: GraphGroup
  change: 'added' | 'removed' | 'unchanged'
  selected: boolean
  dimmed: boolean
  detail?: 'full' | 'compact' | 'minimal'
  onOpen: (id: string) => void
  onSelect: (id: string) => void
  onNavigate: (id: string, d: GraphNavigateDirection) => void
  onToggleGroup: (id: string) => void
}

function Handles(): React.JSX.Element {
  return (
    <>
      <Handle type="target" position={Position.Left} isConnectable={false} className="!opacity-0" />
      <Handle type="source" position={Position.Right} isConnectable={false} className="!opacity-0" />
    </>
  )
}

function GraphNodeRenderer({ data }: NodeProps): React.JSX.Element {
  const d = data as unknown as NodeData
  return (
    <>
      <Handles />
      <GraphNodeCard
        node={d.node as GraphNode}
        change={d.change}
        selected={d.selected}
        dimmed={d.dimmed}
        detail={d.detail}
        onOpen={d.onOpen}
        onSelect={d.onSelect}
        onNavigate={d.onNavigate}
      />
    </>
  )
}

function GraphGroupRenderer({ data }: NodeProps): React.JSX.Element {
  const d = data as unknown as NodeData
  return (
    <>
      <Handles />
      <div className={d.dimmed ? 'opacity-30' : undefined}>
        <GraphGroupNode group={d.group as GraphGroup} onToggleGroup={d.onToggleGroup} />
      </div>
    </>
  )
}

const NODE_TYPES = { graphNode: GraphNodeRenderer, graphGroup: GraphGroupRenderer }
const EDGE_TYPES = { graphEdge: GraphEdgeLine }

export type GraphCanvasProps = {
  payload: GraphPayload
  layout?: LayoutEngine
  selectedId: string | null
  onSelect: (id: string | null) => void
  onOpenNode: (id: string) => void
  openGroups: ReadonlySet<string>
  onToggleGroup: (id: string) => void
  changeView: GraphChangeView
  focusId: string | null
  onFocus: (id: string | null) => void
  fitViewSignal?: number
  className?: string
}

export function GraphCanvas({
  payload, layout = waveLayoutEngine, selectedId, onSelect, onOpenNode, openGroups, onToggleGroup,
  changeView, focusId, onFocus, fitViewSignal = 0, className
}: GraphCanvasProps): React.JSX.Element {
  const colorMode = useDocumentColorMode()
  const reduced = usePrefersReducedMotion()
  const flowRef = useRef<ReactFlowInstance | null>(null)
  const [zoomLevel, setZoomLevel] = useState<GraphZoomLevel>('module')
  const zoomPres = zoomPresentation(zoomLevel)
  const zoomTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const layoutCache = useRef<Map<string, LayoutPositions>>(new Map())
  const [positions, setPositions] = useState<LayoutPositions>({})

  const view = useMemo(() => applyChangeView(payload, changeView), [payload, changeView])
  const { visible, groups } = useMemo(
    () => pickVisibleNodes(view, { selectedId, openGroups }),
    [view, selectedId, openGroups]
  )
  const viewEdges = useMemo(() => buildGroupEdges(view, visible, groups), [view, visible, groups])
  const focusSet = useMemo(() => (focusId ? focusNeighborhood(viewEdges, focusId) : null), [focusId, viewEdges])

  const layoutNodes = useMemo<GraphNode[]>(
    () => [
      ...visible,
      ...groups.map((g) => ({
        id: `${GROUP_NODE_PREFIX}${g.id}`, kind: 'group', label: g.label, group: g.id, risk: g.maxRisk, status: null
      }))
    ],
    [visible, groups]
  )

  const cacheKey = useMemo(
    () => layoutCacheKey(`${payload.nodes.length}:${layoutNodes.map((n) => n.id).join(',')}:${changeView}`, payload.lens, [...openGroups].sort().join(',')),
    [payload, layoutNodes, changeView, openGroups]
  )

  useEffect(() => {
    let cancelled = false
    const hit = layoutCache.current.get(cacheKey)
    if (hit) {
      setPositions(hit)
      return
    }
    const groupOf = (id: string): string | null => layoutNodes.find((n) => n.id === id)?.group ?? null
    void layout(layoutNodes, viewEdges, { direction: 'LR', groupOf }).then((p) => {
      if (cancelled) {return}
      layoutCache.current.set(cacheKey, p)
      while (layoutCache.current.size > LAYOUT_CACHE_MAX) {
        const oldest = layoutCache.current.keys().next().value
        if (oldest === undefined) {break}
        layoutCache.current.delete(oldest)
      }
      setPositions(p)
    })
    return () => { cancelled = true }
  }, [cacheKey, layout, layoutNodes, viewEdges])

  useEffect(() => {
    flowRef.current?.fitView({ padding: 0.2, duration: reduced ? 0 : 200 })
  }, [positions, fitViewSignal, reduced])

  useEffect(() => () => { if (zoomTimer.current) {clearTimeout(zoomTimer.current)} }, [])

  const onNavigate = useCallback(
    (id: string, dir: GraphNavigateDirection) => {
      const pos = positions[id]
      let next: string | undefined
      if (dir === 'left') {next = viewEdges.find((e) => e.to === id)?.from}
      else if (dir === 'right') {next = viewEdges.find((e) => e.from === id)?.to}
      else if (pos) {
        const column = Object.entries(positions)
          .filter(([, p]) => p.x === pos.x)
          .sort(([, a], [, b]) => a.y - b.y)
          .map(([k]) => k)
        const at = column.indexOf(id)
        next = column[dir === 'up' ? at - 1 : at + 1]
      }
      if (!next) {return}
      if (!next.startsWith(GROUP_NODE_PREFIX)) {onSelect(next)}
      // Why: keyboard users need real DOM focus to follow the selection.
      requestAnimationFrame(() => {
        document.querySelector<HTMLElement>(`[data-graph-node-id="${CSS.escape(next as string)}"], [data-graph-group-id="${CSS.escape(next.replace(GROUP_NODE_PREFIX, ''))}"]`)?.focus()
      })
    },
    [positions, viewEdges, onSelect]
  )

  const changeOf = useMemo(() => {
    const m = new Map<string, 'added' | 'removed'>()
    for (const e of view.edges) {
      if (e.change === 'unchanged') {continue}
      m.set(e.from, m.get(e.from) ?? e.change)
      m.set(e.to, m.get(e.to) ?? e.change)
    }
    return m
  }, [view])

  const nodes = useMemo<Node[]>(() => {
    const base = { onOpen: onOpenNode, onSelect, onNavigate, onToggleGroup }
    const out: Node[] = visible.map((n) => ({
      id: n.id,
      type: 'graphNode',
      position: positions[n.id] ?? { x: 0, y: 0 },
      draggable: false,
      data: { ...base, node: n, change: changeOf.get(n.id) ?? 'unchanged', selected: n.id === selectedId, dimmed: isDimmed(n.id, focusSet), detail: zoomPres.nodeDetail }
    }))
    for (const g of groups) {
      const id = `${GROUP_NODE_PREFIX}${g.id}`
      out.push({
        id, type: 'graphGroup', position: positions[id] ?? { x: 0, y: 0 }, draggable: false,
        data: { ...base, group: g, change: 'unchanged', selected: false, dimmed: isDimmed(id, focusSet) }
      })
    }
    return out
  }, [visible, groups, positions, selectedId, focusSet, changeOf, zoomPres.nodeDetail, onOpenNode, onSelect, onNavigate, onToggleGroup])

  const edges = useMemo<Edge[]>(
    () =>
      viewEdges.map((e, i) => ({
        id: `${e.from}->${e.to}:${e.kind}:${i}`,
        source: e.from,
        target: e.to,
        type: 'graphEdge',
        animated: false,
        data: { change: e.change, dim: e.dim === true || (focusSet !== null && !(focusSet.has(e.from) && focusSet.has(e.to))), weight: e.weight, running: payload.lens === 'execution', showSign: zoomPres.showEdgeSigns, opacityFactor: zoomPres.edgeOpacityFactor }
      })),
    [viewEdges, focusSet, payload.lens, zoomPres.showEdgeSigns, zoomPres.edgeOpacityFactor]
  )

  return (
    <div
      className={className ?? 'h-full min-h-[320px] w-full'}
      data-testid="graph-canvas"
      data-zoom-level={zoomLevel}
      onKeyDown={(e) => {
        if (e.key === 'Escape' && focusId) {
          e.stopPropagation()
          onFocus(null)
        }
      }}
    >
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        edgeTypes={EDGE_TYPES}
        colorMode={colorMode}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        onlyRenderVisibleElements={nodes.length > ONLY_VISIBLE_THRESHOLD}
        onInit={(instance) => { flowRef.current = instance }}
        onNodeClick={(_e, node) => { if (!node.id.startsWith(GROUP_NODE_PREFIX)) {onSelect(node.id)} }}
        onNodeDoubleClick={(_e, node) => { if (!node.id.startsWith(GROUP_NODE_PREFIX)) {onFocus(node.id)} }}
        onPaneClick={() => onSelect(null)}
        onMove={(_e, viewport) => {
          if (zoomTimer.current) {clearTimeout(zoomTimer.current)}
          zoomTimer.current = setTimeout(() => { setZoomLevel(levelForZoom(viewport.zoom)) }, ZOOM_DEBOUNCE_MS)
        }}
        fitView
        fitViewOptions={{ padding: 0.2, duration: reduced ? 0 : 200 }}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        {nodes.length > MINIMAP_NODE_THRESHOLD ? <MiniMap nodeStrokeWidth={2} pannable /> : null}
      </ReactFlow>
    </div>
  )
}

export default GraphCanvas
