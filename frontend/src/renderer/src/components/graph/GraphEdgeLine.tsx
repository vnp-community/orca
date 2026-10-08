/**
 * GraphEdgeLine — FE-REQ-TASK-032-05 (custom xyflow edge)
 *
 * @module components/graph/GraphEdgeLine
 */

import React from 'react'
import { BaseEdge, EdgeLabelRenderer, getBezierPath, type EdgeProps } from '@xyflow/react'
import { translate } from '@/i18n/i18n'
import { usePrefersReducedMotion } from '../../hooks/usePrefersReducedMotion'
import { edgePresentation } from './graph-edge-presentation'
import type { GraphChange } from '../../../../shared/graph-types'

export type GraphEdgeData = { change?: GraphChange; dim?: boolean; weight?: number; running?: boolean; showSign?: boolean; opacityFactor?: number }

export function GraphEdgeLine(props: EdgeProps): React.JSX.Element {
  const { id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data } = props
  const edge = (data ?? {}) as GraphEdgeData
  const reduced = usePrefersReducedMotion()
  const pres = edgePresentation(edge.change ?? 'unchanged')
  const [path, labelX, labelY] = getBezierPath({ sourceX, sourceY, sourcePosition, targetX, targetY, targetPosition })
  const label = pres.labelKey ? translate(pres.labelKey, pres.labelFallback ?? '') : null

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        style={{
          stroke: pres.stroke,
          strokeWidth: 1.5,
          strokeDasharray: pres.strokeDasharray,
          opacity: (edge.dim ? Math.min(pres.opacity, 0.5) : pres.opacity) * (edge.opacityFactor ?? 1)
        }}
        className={edge.running && !reduced ? 'react-flow__edge-path animated' : undefined}
      />
      {pres.sign && edge.showSign !== false ? (
        <EdgeLabelRenderer>
          <span
            aria-label={label ?? undefined}
            className="pointer-events-none absolute rounded bg-background px-1 font-mono text-[10px] text-foreground"
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
          >
            {pres.sign}
          </span>
        </EdgeLabelRenderer>
      ) : null}
    </>
  )
}
