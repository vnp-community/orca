/**
 * StorageCanvas.tsx — FE-CV-TASK-058-04
 *
 * Read-only xyflow canvas over the four fixed lanes. Colours come from CSS variables.
 */

import { useMemo } from 'react'
import {
  Background,
  Controls,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Edge,
  type Node,
  type NodeProps
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Database, Radio, Server } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { isEditableTarget } from '@/lib/editable-target'
import { useDocumentColorMode } from '../../../hooks/useDocumentColorMode'
import { usePrefersReducedMotion } from '../../../hooks/usePrefersReducedMotion'
import { ERD_CHANGE_TEXT_CLASS } from '../erd/erd-change-style'
import { STORAGE_MARK_SYMBOL, type StorageMark } from './storage-change-marks'
import { STORAGE_NODE_HEIGHT, STORAGE_NODE_WIDTH, type StorageRect } from './storage-layout'
import { StorageSecretNode, type StorageNodeData } from './StorageSecretNode'
import type { StorageEdge, StorageNode } from './storage-view-model'

const ICON = { service: Server, store: Database, topic: Radio } as const

function StorageCardNode({ data, selected }: NodeProps): React.JSX.Element {
  const { node, mark, dimmed } = data as StorageNodeData
  const Icon = ICON[node.lane as keyof typeof ICON] ?? Database
  const markLabel = mark
    ? translate(`auto.components.reviewMap.StorageCanvas.mark.${mark}`, mark)
    : null
  return (
    <div
      role="group"
      tabIndex={0}
      aria-label={`${node.name}${markLabel ? `, ${markLabel}` : ''}`}
      data-testid={`storage-node-${node.id}`}
      className={cn(
        'rounded-md border bg-card px-2 py-1.5 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring',
        node.external && 'border-dashed',
        mark === 'related' && 'border-muted-foreground/50',
        dimmed && 'opacity-40',
        selected && 'ring-2 ring-ring'
      )}
      style={{ width: STORAGE_NODE_WIDTH, height: STORAGE_NODE_HEIGHT }}
    >
      <div className="flex items-center gap-1 font-medium">
        <Icon className="size-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{node.name}</span>
        {mark && mark !== 'related' ? (
          <span className={cn('ml-auto font-mono', ERD_CHANGE_TEXT_CLASS[mark])} aria-hidden="true">
            {STORAGE_MARK_SYMBOL[mark]}
          </span>
        ) : null}
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-1 text-[10px]">
        {node.kind ? <Badge variant="outline">{node.kind}</Badge> : null}
        {node.confidence === 'inferred' || node.confidence === 'derived' ? (
          <Badge variant="secondary">
            {translate(
              `auto.components.reviewMap.StorageCanvas.confidence.${node.confidence}`,
              node.confidence
            )}
          </Badge>
        ) : null}
        {node.deployed === false ? (
          <span>
            {translate('auto.components.reviewMap.StorageCanvas.notDeployed', 'not deployed')}
          </span>
        ) : null}
        {node.supportedByCode === false ? (
          <span>
            {translate('auto.components.reviewMap.StorageCanvas.noCodeSupport', 'no code support')}
          </span>
        ) : null}
        {node.external ? (
          <span>{translate('auto.components.reviewMap.StorageCanvas.external', 'external')}</span>
        ) : null}
      </div>
    </div>
  )
}

// Declared once: xyflow remounts nodes when this object's identity changes.
const NODE_TYPES = {
  storageService: StorageCardNode,
  storageStore: StorageCardNode,
  storageTopic: StorageCardNode,
  storageSecret: StorageSecretNode
}
const TYPE_BY_LANE = {
  service: 'storageService',
  store: 'storageStore',
  topic: 'storageTopic',
  secret: 'storageSecret'
} as const

const EDGE_DASH: Record<StorageEdge['kind'], string | undefined> = {
  binding: undefined,
  publish: '6 3',
  subscribe: '3 3',
  'config-key': '2 3'
}

export type StorageCanvasProps = {
  nodes: readonly StorageNode[]
  edges: readonly StorageEdge[]
  positions: ReadonlyMap<string, StorageRect>
  marks: ReadonlyMap<string, StorageMark>
  selectedId: string | null
  onSelect: (id: string) => void
}

function Inner({
  nodes,
  edges,
  positions,
  marks,
  selectedId,
  onSelect
}: StorageCanvasProps): React.JSX.Element {
  const colorMode = useDocumentColorMode()
  const reduceMotion = usePrefersReducedMotion()
  const flow = useReactFlow()
  const anyMark = marks.size > 0

  const flowNodes = useMemo<Node[]>(
    () =>
      nodes.flatMap((n) => {
        const pos = positions.get(n.id)
        if (!pos) {
          return []
        }
        const mark = marks.get(n.id)
        return [
          {
            id: n.id,
            type: TYPE_BY_LANE[n.lane],
            position: { x: pos.x, y: pos.y },
            selected: n.id === selectedId,
            data: { node: n, mark, dimmed: anyMark && !mark } satisfies StorageNodeData
          }
        ]
      }),
    [nodes, positions, marks, selectedId, anyMark]
  )
  const flowEdges = useMemo<Edge[]>(
    () =>
      edges
        .filter((e) => positions.has(e.from) && positions.has(e.to))
        .map((e) => ({
          id: e.id,
          source: e.from,
          target: e.to,
          label:
            e.kind === 'binding'
              ? e.access
              : e.kind === 'publish'
                ? 'pub'
                : e.kind === 'subscribe'
                  ? 'sub'
                  : undefined,
          style: {
            stroke:
              marks.has(e.id) && marks.get(e.id) !== 'related'
                ? 'var(--review-changed)'
                : 'var(--muted-foreground)',
            strokeDasharray: EDGE_DASH[e.kind],
            strokeWidth: 1.25,
            opacity: e.confidence === 'inferred' ? 0.6 : 1
          }
        })),
    [edges, positions, marks]
  )

  // Why: 'f' is a lens-local shortcut, active only while focus is inside the canvas.
  return (
    <div
      className="min-h-0 flex-1"
      role="img"
      aria-label={translate(
        'auto.components.reviewMap.StorageCanvas.label',
        'Storage map; use the text view for an equivalent'
      )}
      onKeyDown={(e) => {
        if (e.key === 'f' && !isEditableTarget(e.target) && !e.metaKey && !e.ctrlKey && !e.altKey) {
          void flow.fitView({ padding: 0.15, duration: reduceMotion ? 0 : 200 })
        }
      }}
    >
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={NODE_TYPES}
        colorMode={colorMode}
        nodesDraggable={false}
        nodesConnectable={false}
        onlyRenderVisibleElements
        fitView
        fitViewOptions={{ padding: 0.15, duration: reduceMotion ? 0 : 200 }}
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_e, node) => onSelect(node.id)}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}

export function StorageCanvas(props: StorageCanvasProps): React.JSX.Element {
  return (
    <ReactFlowProvider>
      <Inner {...props} />
    </ReactFlowProvider>
  )
}
