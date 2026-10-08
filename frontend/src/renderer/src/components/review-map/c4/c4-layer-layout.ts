/**
 * c4-layer-layout.ts — FE-CV-TASK-055-01
 *
 * Fixed hexagonal column layout of a C4 component view. Why fixed (no force layout): the same
 * data must always land on the same coordinates so a reviewer's spatial memory survives reloads.
 */

import type { C4Component, C4ComponentView, C4External, C4Relation } from '../../../../../shared/code-intel-architecture-types'

export const C4_NODE_WIDTH = 208
export const C4_NODE_HEIGHT = 76
export const C4_COLUMN_GAP = 88
export const C4_ROW_GAP = 28
export const C4_BAND_PADDING = 12

export type C4LayerId = 'grpc-server' | 'usecase' | 'domain' | 'adapter' | 'external' | 'support'

/** Column index of each layer; adapters and gRPC clients share the infrastructure column. */
const LAYER_COLUMN: Record<C4LayerId, number> = {
  'grpc-server': 0,
  usecase: 1,
  domain: 2,
  adapter: 3,
  external: 4,
  support: 1
}

export function c4LayerForComponentKind(kind: string): C4LayerId {
  switch (kind) {
    case 'grpc-server':
      return 'grpc-server'
    case 'usecase':
      return 'usecase'
    case 'domain':
      return 'domain'
    case 'adapter':
    case 'grpc-client':
      return 'adapter'
    default:
      // config / other / unknown wire values all go below the use-case column.
      return 'support'
  }
}

export type C4LayoutNode =
  | { id: string; type: 'component'; layer: C4LayerId; x: number; y: number; component: C4Component }
  | { id: string; type: 'external'; layer: 'external'; x: number; y: number; external: C4External }

export type C4LayoutBand = {
  id: string
  layer: C4LayerId
  /** i18n suffix under ...reviewMap.c4.layer.* */
  x: number
  y: number
  width: number
  height: number
}

export type C4Layout = {
  nodes: C4LayoutNode[]
  bands: C4LayoutBand[]
  /** Relations whose both endpoints exist, in input order. */
  relations: C4Relation[]
  droppedRelations: number
}

const LAYER_ORDER: C4LayerId[] = ['grpc-server', 'usecase', 'domain', 'adapter', 'external', 'support']

export function layoutC4Layers(
  view: Pick<C4ComponentView, 'components' | 'externals' | 'relations'>
): C4Layout {
  const byLayer = new Map<C4LayerId, (C4Component | C4External)[]>()
  const push = (layer: C4LayerId, item: C4Component | C4External): void => {
    const list = byLayer.get(layer) ?? []
    list.push(item)
    byLayer.set(layer, list)
  }
  // Why sort by name then id: input order from the server is not guaranteed stable.
  const components = [...view.components].sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id))
  const externals = [...view.externals].sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id))
  for (const c of components) {
    push(c4LayerForComponentKind(c.kind), c)
  }
  for (const e of externals) {
    push('external', e)
  }

  const colX = (column: number): number => column * (C4_NODE_WIDTH + C4_COLUMN_GAP)
  const rowY = (row: number): number => row * (C4_NODE_HEIGHT + C4_ROW_GAP)

  const nodes: C4LayoutNode[] = []
  const bands: C4LayoutBand[] = []
  // Support nodes stack beneath the tallest of the use-case column, so compute that first.
  const usecaseRows = byLayer.get('usecase')?.length ?? 0
  // adapter column and grpc-client share, so adapter layer already merges them.
  const seen = new Set<string>()

  for (const layer of LAYER_ORDER) {
    const items = byLayer.get(layer)
    if (!items || items.length === 0) {
      continue
    }
    const column = LAYER_COLUMN[layer]
    const startRow = layer === 'support' ? usecaseRows + (usecaseRows > 0 ? 1 : 0) : 0
    items.forEach((item, i) => {
      const y = rowY(startRow + i)
      if (seen.has(item.id)) {
        return
      }
      seen.add(item.id)
      if (layer === 'external') {
        nodes.push({ id: item.id, type: 'external', layer, x: colX(column), y, external: item as C4External })
      } else {
        nodes.push({ id: item.id, type: 'component', layer, x: colX(column), y, component: item as C4Component })
      }
    })
    if (layer !== 'support') {
      bands.push({
        id: `band-${layer}`,
        layer,
        x: colX(column) - C4_BAND_PADDING,
        y: -C4_BAND_PADDING,
        width: C4_NODE_WIDTH + C4_BAND_PADDING * 2,
        height: Math.max(items.length, 1) * (C4_NODE_HEIGHT + C4_ROW_GAP) - C4_ROW_GAP + C4_BAND_PADDING * 2
      })
    }
  }

  const nodeIds = new Set(nodes.map((n) => n.id))
  const relations = view.relations.filter((r) => nodeIds.has(r.from) && nodeIds.has(r.to))
  return { nodes, bands, relations, droppedRelations: view.relations.length - relations.length }
}
