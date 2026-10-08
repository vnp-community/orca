/** StorageTextView.tsx — FE-CV-TASK-058-04. Text equivalent of the canvas (service -> store -> access). */

import { translate } from '@/i18n/i18n'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { STORAGE_MARK_SYMBOL, type StorageMark } from './storage-change-marks'
import type { StorageEdge, StorageNode } from './storage-view-model'

export function StorageTextView({
  nodes,
  edges,
  marks,
  onSelect
}: {
  nodes: readonly StorageNode[]
  edges: readonly StorageEdge[]
  marks: ReadonlyMap<string, StorageMark>
  onSelect: (id: string) => void
}): React.JSX.Element {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const t = translate
  return (
    <details className="px-3 py-1 text-xs">
      <summary className="cursor-pointer">
        {t('auto.components.reviewMap.StorageTextView.summary', 'Text view')}
      </summary>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('auto.components.reviewMap.StorageTextView.from', 'From')}</TableHead>
            <TableHead>
              {t('auto.components.reviewMap.StorageTextView.relation', 'Relation')}
            </TableHead>
            <TableHead>{t('auto.components.reviewMap.StorageTextView.to', 'To')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {edges.map((e) => {
            const from = byId.get(e.from)
            const to = byId.get(e.to)
            if (!from || !to) {
              return null
            }
            const mark = marks.get(e.id)
            return (
              <TableRow key={e.id}>
                <TableCell>
                  <button
                    type="button"
                    className="underline-offset-2 hover:underline"
                    onClick={() => onSelect(from.id)}
                  >
                    {from.name}
                  </button>
                </TableCell>
                <TableCell>
                  {mark ? <span aria-hidden="true">{STORAGE_MARK_SYMBOL[mark]} </span> : null}
                  {e.kind === 'binding' ? e.access : e.kind}
                </TableCell>
                <TableCell>
                  <button
                    type="button"
                    className="underline-offset-2 hover:underline"
                    onClick={() => onSelect(to.id)}
                  >
                    {to.name}
                  </button>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </details>
  )
}
