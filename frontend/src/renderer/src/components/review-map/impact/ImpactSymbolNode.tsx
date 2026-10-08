import { Handle, Position } from '@xyflow/react'
import type { NodeProps } from '@xyflow/react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { ReviewNodeNoteBadge } from '../notes/ReviewNodeNoteBadge'
import { ImpactNodeCard } from './ImpactNodeCard'
import type { ImpactNodeActions } from './ImpactNodeCard'
import { turnOverlayDimmed } from '../review-overlay-model'
import type { OverlayFlagSet } from '../review-overlay-model'
import { turnChangeLabel } from '../turns/ReviewTurnSwitcher'
import type { TurnChangeLabel } from '../turns/turn-compare-model'
import type { SymbolRefView } from '../review-wire-types'

export type ImpactFlowNodeData = {
  kind: 'center' | 'symbol' | 'more'
  symbol?: SymbolRefView
  flags: OverlayFlagSet
  selected: boolean
  direct?: boolean
  via?: string
  hiddenCount?: number
  column: number
  actions: ImpactNodeActions
  onExpand: (column: number) => void
  /** Unsent review notes anchored on this symbol. */
  noteCount?: number
  /** "Since previous turn" label; null outside that view. */
  turnLabel?: TurnChangeLabel | null
  [key: string]: unknown
}

export function ImpactSymbolNode({ data }: NodeProps): React.JSX.Element {
  const d = data as ImpactFlowNodeData
  return (
    <div
      className={cn('relative', turnOverlayDimmed(d.turnLabel ?? null) && 'opacity-50')}
      style={{ width: 260 }}
      data-turn-label={d.turnLabel ?? undefined}
    >
      <Handle type="target" position={Position.Left} isConnectable={false} />
      {d.kind === 'more' ? (
        <Button type="button" size="sm" variant="outline" onClick={() => d.onExpand(d.column)}>
          {translate('auto.components.reviewMap.impact.more', '+{{count}} more', {
            count: d.hiddenCount ?? 0
          })}
        </Button>
      ) : (
        <ImpactNodeCard
          symbol={d.symbol!}
          flags={d.flags}
          selected={d.selected}
          isCenter={d.kind === 'center'}
          direct={d.direct}
          via={d.via}
          actions={d.actions}
        />
      )}
      {d.turnLabel ? (
        <span className="pointer-events-none absolute -top-2 left-2 rounded-full border border-border bg-background px-1.5 text-[10px] text-muted-foreground">
          {turnChangeLabel(d.turnLabel)}
        </span>
      ) : null}
      {d.noteCount ? (
        <div className="pointer-events-none absolute -right-1.5 -top-1.5">
          <ReviewNodeNoteBadge count={d.noteCount} />
        </div>
      ) : null}
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  )
}
