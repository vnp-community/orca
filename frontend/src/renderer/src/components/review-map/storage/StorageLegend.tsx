/** StorageLegend.tsx — FE-CV-TASK-058-04. Symbols and line styles, same table as the canvas. */

import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { ERD_CHANGE_TEXT_CLASS } from '../erd/erd-change-style'
import { STORAGE_MARK_SYMBOL } from './storage-change-marks'

export function StorageLegend(): React.JSX.Element {
  const marks = ['added', 'modified', 'removed'] as const
  const label = (k: string, fallback: string): string =>
    translate(`auto.components.reviewMap.StorageLegend.${k}`, fallback)
  return (
    <ul
      className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground"
      aria-label={label('label', 'Legend')}
    >
      {marks.map((m) => (
        <li key={m} className={cn(ERD_CHANGE_TEXT_CLASS[m])}>
          <span className="font-mono" aria-hidden="true">
            {STORAGE_MARK_SYMBOL[m]}
          </span>{' '}
          {label(m, m)}
        </li>
      ))}
      <li>
        <span aria-hidden="true">{STORAGE_MARK_SYMBOL.related}</span>{' '}
        {label('related', 'related to a change')}
      </li>
      <li>
        <span aria-hidden="true">━</span> {label('binding', 'read/write (rw) or read-only (ro)')}
      </li>
      <li>
        <span aria-hidden="true">╌</span> {label('topic', 'publish / subscribe')}
      </li>
      <li>
        <span aria-hidden="true">┄</span> {label('configKey', 'secret reference')}
      </li>
    </ul>
  )
}
