/**
 * OutcomeDiffRow — FE-OR-T203
 *
 * One row of a section: a symbol (+ ~ − ∅, never colour alone), the title and detail, the flags, the evidence, and three dots
 * that say whether the thing is in the design, on the base and on the head. A row an agent wrote says "Agent reported".
 *
 * @module components/request/outcome-review/diff/OutcomeDiffRow
 */

import React from 'react'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { cn } from '@/lib/utils'
import { agentReportedText, changeText } from '../outcome-review-copy'
import { dotLetter, dotText, flagText, seenText } from './outcome-diff-copy'
import { OutcomeEvidenceChip } from './OutcomeEvidenceChip'
import type { DiffRowModel, Tri } from './outcome-diff-view-model'

const DOT_TONE: Record<Tri, string> = {
  yes: 'bg-foreground text-background',
  no: 'border border-border text-muted-foreground',
  unknown: 'border border-dashed border-border text-muted-foreground'
}

function Dot({
  where,
  value
}: {
  where: 'design' | 'base' | 'head'
  value: Tri
}): React.JSX.Element {
  return (
    <span
      role="img"
      aria-label={dotText(where, value)}
      title={dotText(where, value)}
      className={cn(
        'inline-flex size-4 items-center justify-center rounded-full text-[9px] font-medium',
        DOT_TONE[value]
      )}
      data-where={where}
      data-value={value}
    >
      {dotLetter(where)}
    </span>
  )
}

type Props = {
  row: DiffRowModel
  /** Absent hides the dots (before the code exists there is no base or head). */
  showDots: boolean
  seen: boolean
  onSeenChange: (seen: boolean) => void
  onOpenFile?: (path: string) => void
  /** What goes at the end of the row: the way into the code review. */
  actions?: React.ReactNode
}

export function OutcomeDiffRow({
  row,
  showDots,
  seen,
  onSeenChange,
  onOpenFile,
  actions
}: Props): React.JSX.Element {
  const { item } = row
  return (
    <li
      className={cn(
        'flex items-start gap-3 rounded-md border border-border/60 p-2',
        seen && 'opacity-70'
      )}
      data-testid="outcome-diff-row"
      data-item-key={item.key}
      data-symbol={row.symbol}
    >
      <span
        className="w-5 shrink-0 text-center font-mono text-base leading-5"
        role="img"
        aria-label={changeText(item.change)}
        data-testid="outcome-diff-symbol"
      >
        {row.symbol}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium">{item.title}</span>
          {row.agentReported && <Badge variant="outline">{agentReportedText()}</Badge>}
          {row.flags.map((flag) => (
            <Badge
              key={flag}
              variant={flag === 'breaking' || flag === 'regression' ? 'destructive' : 'secondary'}
              data-flag={flag}
            >
              {flagText(flag)}
            </Badge>
          ))}
        </div>
        {item.detail && <p className="text-xs text-muted-foreground">{item.detail}</p>}
        {item.protocol && <p className="text-xs text-muted-foreground">{item.protocol}</p>}
        {item.evidence.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {item.evidence.map((e, i) => (
              <OutcomeEvidenceChip
                key={`${e.kind}:${e.ref}:${i}`}
                evidence={e}
                onOpenFile={onOpenFile}
              />
            ))}
          </div>
        )}
      </div>
      {showDots && (
        <span className="flex shrink-0 items-center gap-1" data-testid="outcome-diff-dots">
          <Dot where="design" value={row.dots.design} />
          <Dot where="base" value={row.dots.base} />
          <Dot where="head" value={row.dots.head} />
        </span>
      )}
      {actions && <span className="shrink-0">{actions}</span>}
      <label className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
        <Checkbox
          checked={seen}
          onCheckedChange={(v) => onSeenChange(v === true)}
          aria-label={`${seenText()}: ${item.title}`}
        />
        {seenText()}
      </label>
    </li>
  )
}
