/** C4EdgeLegend.tsx — FE-CV-TASK-055-03: legend built from the same table the edges use. */

import { translate } from '@/i18n/i18n'
import { C4_RELATION_DASH, C4_RELATION_KINDS } from './c4-edge-style'

export function c4RelationKindLabel(kind: string): string {
  return translate(`auto.components.reviewMap.c4.relation.${C4_RELATION_KINDS.includes(kind as never) ? kind : 'unknown'}`, kind)
}

export function C4EdgeLegend(): React.JSX.Element {
  return (
    <ul aria-label={translate('auto.components.reviewMap.c4.legend', 'Legend')} className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
      {C4_RELATION_KINDS.map((kind) => (
        <li key={kind} className="flex items-center gap-1.5">
          <svg width="28" height="8" aria-hidden="true">
            <line x1="0" y1="4" x2="28" y2="4" stroke="currentColor" strokeWidth="2" strokeDasharray={C4_RELATION_DASH[kind]} />
          </svg>
          {c4RelationKindLabel(kind)}
        </li>
      ))}
    </ul>
  )
}
