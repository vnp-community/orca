/**
 * Edge style per change kind: sign, dash and token color.
 *
 * @module components/graph/graph-edge-presentation
 */

import type { GraphChange } from '../../../../shared/graph-types'

export type EdgePresentation = {
  stroke: string
  strokeDasharray?: string
  opacity: number
  sign: '+' | '−' | null
  labelKey: string | null
  labelFallback: string | null
}

export function edgePresentation(change: GraphChange): EdgePresentation {
  if (change === 'added') {
    return { stroke: 'var(--graph-edge-added)', opacity: 1, sign: '+', labelKey: 'auto.components.graph.Edge.added', labelFallback: 'Added' }
  }
  if (change === 'removed') {
    return {
      stroke: 'var(--graph-edge-removed)', strokeDasharray: '6 4', opacity: 0.5, sign: '−',
      labelKey: 'auto.components.graph.Edge.removed', labelFallback: 'Removed'
    }
  }
  return { stroke: 'var(--border)', opacity: 1, sign: null, labelKey: null, labelFallback: null }
}
