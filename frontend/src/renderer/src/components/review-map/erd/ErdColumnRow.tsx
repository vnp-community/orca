/**
 * ErdColumnRow.tsx — FE-CV-TASK-057-05
 *
 * One column line. Change state is a symbol plus a screen-reader label, never colour alone.
 */

import { KeyRound, Link2, ShieldAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import type { ErdColumnView } from './erd-column-changes'
import { ERD_CHANGE_LABEL, ERD_CHANGE_TEXT_CLASS } from './erd-change-style'

export function ErdColumnRow({ column }: { column: ErdColumnView }): React.JSX.Element {
  const change = column.change
  const label = change
    ? translate(ERD_CHANGE_LABEL[change].key, ERD_CHANGE_LABEL[change].fallback)
    : null
  return (
    <li
      className={cn(
        'flex h-[22px] items-center gap-1 px-2 text-[11px]',
        change && ERD_CHANGE_TEXT_CLASS[change],
        column.ghost && 'line-through'
      )}
      data-testid={`erd-col-${column.name}`}
    >
      <span className="w-3 shrink-0 text-center font-mono" aria-hidden={label ? undefined : true}>
        {column.changeSymbol ?? ''}
      </span>
      {label ? <span className="sr-only">{label}</span> : null}
      {column.isPk ? (
        <KeyRound
          className="size-3 shrink-0"
          aria-label={translate('auto.components.reviewMap.ErdColumnRow.primaryKey', 'Primary key')}
        />
      ) : column.isFk ? (
        <Link2
          className="size-3 shrink-0"
          aria-label={translate('auto.components.reviewMap.ErdColumnRow.foreignKey', 'Foreign key')}
        />
      ) : (
        <span className="size-3 shrink-0" aria-hidden="true" />
      )}
      <span className="truncate font-medium">{column.name}</span>
      <span className="ml-auto truncate text-muted-foreground">
        {change === 'modified' && column.before && column.after
          ? `${column.before.type} → ${column.after.type}`
          : column.canonicalType || column.type}
      </span>
      {column.masked ? (
        <ShieldAlert
          className="size-3 shrink-0 text-muted-foreground"
          aria-label={translate(
            'auto.components.reviewMap.ErdColumnRow.masked',
            'Sensitive text was masked'
          )}
        />
      ) : null}
    </li>
  )
}
