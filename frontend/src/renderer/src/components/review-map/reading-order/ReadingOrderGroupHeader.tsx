import { ChevronDown, ChevronRight } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { translate } from '@/i18n/i18n'
import type { ReadingOrderGroupRow } from '../reading-order-model'

type Props = {
  row: ReadingOrderGroupRow
  active: boolean
  domId: string
  onToggleCollapsed: () => void
  onToggleSeen: () => void
  onSelect: () => void
}

export function ReadingOrderGroupHeader({
  row,
  active,
  domId,
  onToggleCollapsed,
  onToggleSeen,
  onSelect
}: Props): React.JSX.Element {
  const label = row.label || translate('auto.components.reviewMap.readingOrder.otherGroup', 'Other')
  const allSeen = row.total > 0 && row.seen === row.total
  return (
    <div
      id={domId}
      role="option"
      aria-selected={active}
      aria-expanded={!row.collapsed}
      data-row="group"
      onClick={onSelect}
      className={`flex h-7 items-center gap-1.5 px-2 text-xs font-medium ${active ? 'bg-accent' : ''}`}
    >
      <button
        type="button"
        tabIndex={-1}
        aria-label={label}
        onClick={onToggleCollapsed}
        className="shrink-0"
      >
        {row.collapsed ? (
          <ChevronRight className="size-3.5" aria-hidden />
        ) : (
          <ChevronDown className="size-3.5" aria-hidden />
        )}
      </button>
      <Checkbox
        tabIndex={-1}
        checked={allSeen ? true : row.seen > 0 ? 'indeterminate' : false}
        onCheckedChange={onToggleSeen}
        aria-label={translate(
          'auto.components.reviewMap.readingOrder.markGroup',
          'Mark {{group}} as read',
          { group: label }
        )}
      />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span className="text-muted-foreground">
        {row.seen}/{row.total}
      </span>
    </div>
  )
}
