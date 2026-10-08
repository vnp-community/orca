/**
 * C4RelationsTable.tsx — FE-CV-TASK-055-03
 *
 * Text equivalent of the diagram; also the default view for very large containers.
 */

import { TriangleAlert } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { translate } from '@/i18n/i18n'
import type { C4Relation } from '../../../../../shared/code-intel-architecture-types'
import { c4RelationKindLabel } from './C4EdgeLegend'

export function relationKey(r: Pick<C4Relation, 'from' | 'to' | 'kind'>): string {
  return `${r.from}\u0000${r.kind}\u0000${r.to}`
}

export function C4RelationsTable({
  relations,
  names,
  total,
  changedKeys,
  selectedKey,
  onSelect
}: {
  relations: readonly C4Relation[]
  /** component/external id -> display name */
  names: ReadonlyMap<string, string>
  total: number
  changedKeys?: ReadonlySet<string>
  selectedKey?: string | null
  onSelect: (relation: C4Relation) => void
}): React.JSX.Element {
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <p className="px-3 py-1 text-[11px] text-muted-foreground">
        {translate('auto.components.reviewMap.c4.showingCount', 'Showing {{shown}}/{{total}}', {
          shown: relations.length,
          total
        })}
      </p>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{translate('auto.components.reviewMap.c4.col.from', 'From')}</TableHead>
            <TableHead>{translate('auto.components.reviewMap.c4.col.kind', 'Type')}</TableHead>
            <TableHead>{translate('auto.components.reviewMap.c4.col.to', 'To')}</TableHead>
            <TableHead className="text-right">{translate('auto.components.reviewMap.c4.col.count', 'Count')}</TableHead>
            <TableHead>{translate('auto.components.reviewMap.c4.col.flags', 'Flags')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {relations.map((r) => {
            const key = relationKey(r)
            return (
              <TableRow
                key={key}
                data-selected={selectedKey === key || undefined}
                tabIndex={0}
                className="cursor-pointer data-[selected]:bg-accent"
                onClick={() => onSelect(r)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    onSelect(r)
                  }
                }}
              >
                <TableCell>{names.get(r.from) ?? r.from}</TableCell>
                <TableCell>{c4RelationKindLabel(r.kind)}</TableCell>
                <TableCell>{names.get(r.to) ?? r.to}</TableCell>
                <TableCell className="text-right tabular-nums">{r.count}</TableCell>
                <TableCell className="space-x-2 text-xs">
                  {changedKeys?.has(key) ? (
                    <span className="border-l-2 border-review-changed pl-1">
                      {translate('auto.components.reviewMap.c4.flag.changed', 'changed')}
                    </span>
                  ) : null}
                  {r.violatesLayering ? (
                    <span className="inline-flex items-center gap-1 text-destructive">
                      <TriangleAlert className="size-3" aria-hidden="true" />
                      {translate('auto.components.reviewMap.c4.flag.violation', 'layering')}
                    </span>
                  ) : null}
                  {r.confidence < 0.8 ? (
                    <span className="text-muted-foreground">
                      {translate('auto.components.reviewMap.c4.flag.lowConfidence', 'low confidence')}
                    </span>
                  ) : null}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
