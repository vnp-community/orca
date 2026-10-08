/**
 * riskPresentation — FE-REQ-TASK-032-01
 *
 * Text first, shape second, color third: every level has a label key, an icon
 * and a border style so risk never depends on color alone.
 *
 * @module components/graph/risk-presentation
 */

import { CircleCheck, CircleHelp, OctagonAlert, ShieldAlert, TriangleAlert } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { GraphRisk } from '../../../../shared/graph-types'

export type RiskPresentation = {
  level: GraphRisk
  labelKey: string
  labelFallback: string
  Icon: LucideIcon
  borderWidthClass: 'border' | 'border-2'
  doubleRing: boolean
  dashed: boolean
  textClass: string
  bgClass: string
  borderClass: string
}

const LABEL_PREFIX = 'auto.components.graph.RiskLevel.'

const TABLE: Record<GraphRisk, RiskPresentation> = {
  low: {
    level: 'low',
    labelKey: `${LABEL_PREFIX}low`,
    labelFallback: 'Low',
    Icon: CircleCheck,
    borderWidthClass: 'border',
    doubleRing: false,
    dashed: false,
    textClass: 'text-risk-low',
    bgClass: 'bg-risk-low-background',
    borderClass: 'border-risk-low-border'
  },
  medium: {
    level: 'medium',
    labelKey: `${LABEL_PREFIX}medium`,
    labelFallback: 'Medium',
    Icon: TriangleAlert,
    borderWidthClass: 'border',
    doubleRing: false,
    dashed: false,
    textClass: 'text-risk-medium',
    bgClass: 'bg-risk-medium-background',
    borderClass: 'border-risk-medium-border'
  },
  high: {
    level: 'high',
    labelKey: `${LABEL_PREFIX}high`,
    labelFallback: 'High',
    Icon: OctagonAlert,
    borderWidthClass: 'border-2',
    doubleRing: false,
    dashed: false,
    textClass: 'text-risk-high',
    bgClass: 'bg-risk-high-background',
    borderClass: 'border-risk-high'
  },
  critical: {
    level: 'critical',
    labelKey: `${LABEL_PREFIX}critical`,
    labelFallback: 'Critical',
    Icon: ShieldAlert,
    borderWidthClass: 'border-2',
    doubleRing: true,
    dashed: false,
    textClass: 'text-risk-critical',
    bgClass: 'bg-risk-critical-background',
    borderClass: 'border-risk-critical'
  },
  unknown: {
    level: 'unknown',
    labelKey: `${LABEL_PREFIX}unknown`,
    labelFallback: 'Not assessed',
    Icon: CircleHelp,
    borderWidthClass: 'border',
    doubleRing: false,
    dashed: true,
    // Why: unassessed must never borrow a risk color, or it reads as "low".
    textClass: 'text-muted-foreground',
    bgClass: 'bg-muted',
    borderClass: 'border-border'
  }
}

export function riskPresentation(level: GraphRisk): RiskPresentation {
  return Object.prototype.hasOwnProperty.call(TABLE, level) ? TABLE[level] : TABLE.unknown
}
