/**
 * ErdRelationEdge.tsx — FE-CV-TASK-057-05
 *
 * Edge styling for relations (solid = foreign key, dashed = logical) and the one-line
 * provenance label used in the detail panel and as the edge label for inferred links.
 */

import type { Edge } from '@xyflow/react'
import { translate } from '@/i18n/i18n'
import type { ErdRelationView } from './erd-view-model'

export function erdRelationSourceLabel(relation: Pick<ErdRelationView, 'source'>): string {
  switch (relation.source) {
    case 'ddl':
      return translate('auto.components.reviewMap.ErdRelationEdge.source.ddl', 'Declared in DDL')
    case 'declared':
      return translate(
        'auto.components.reviewMap.ErdRelationEdge.source.declared',
        'Declared in erd-links'
      )
    case 'comment':
      return translate(
        'auto.components.reviewMap.ErdRelationEdge.source.comment',
        'From a SQL comment'
      )
    case 'naming':
      return translate(
        'auto.components.reviewMap.ErdRelationEdge.source.naming',
        'Inferred from naming'
      )
    default:
      return translate('auto.components.reviewMap.ErdRelationEdge.source.unknown', 'Unknown source')
  }
}

export function erdEdgeFor(
  relation: ErdRelationView,
  source: string,
  target: string,
  selected: boolean
): Edge {
  return {
    id: relation.id,
    source,
    target,
    selected,
    label: relation.source === 'naming' ? erdRelationSourceLabel(relation) : undefined,
    style: {
      stroke: relation.anomaly ? 'var(--destructive)' : 'var(--muted-foreground)',
      strokeWidth: selected ? 2 : 1.25,
      strokeDasharray: relation.dashed ? '6 4' : undefined
    }
  }
}
