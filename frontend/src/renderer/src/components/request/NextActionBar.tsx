/**
 * NextActionBar — FE-REQ-TASK-037-06
 *
 * One strip under the stage timeline on every tab: what to look at next and a button that goes there or does it. It announces
 * changes politely and never takes focus.
 *
 * @module components/request/NextActionBar
 */

import React from 'react'
import { Check, Hand, Loader2, OctagonX, Clock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { NextAction, NextActionCommand, NextActionKind } from './next-action-rules'

const ICON: Record<Exclude<NextActionKind, 'none'>, React.ComponentType<{ className?: string }>> = {
  failed: OctagonX,
  needs_you: Hand,
  running: Loader2,
  waiting: Clock,
  done: Check
}

// Why: needs-you and running share the primary tone (no new colour token); icon and words tell them apart.
const TONE: Record<Exclude<NextActionKind, 'none'>, string> = {
  failed: 'text-destructive',
  needs_you: 'text-primary',
  running: 'text-primary',
  waiting: 'text-muted-foreground',
  done: 'text-[color:var(--status-success)]'
}

type Props = {
  action: NextAction
  busy?: boolean
  onCommand: (command: NextActionCommand, action: NextAction) => void
}

export function NextActionBar({ action, busy = false, onCommand }: Props): React.JSX.Element {
  // Why: the live region stays mounted while empty so a screen reader announces the text that appears in it.
  if (action.kind === 'none') {
    return <div aria-live="polite" className="sr-only" data-testid="next-action-empty" />
  }
  const Icon = ICON[action.kind]
  return (
    <div
      aria-live="polite"
      className="flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm"
      data-testid="next-action-bar"
      data-kind={action.kind}
    >
      <Icon
        className={cn(
          'size-4 shrink-0',
          TONE[action.kind],
          action.kind === 'running' && 'animate-spin'
        )}
        aria-hidden
      />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="font-medium">{action.title}</span>
        {action.detail && <span className="text-xs text-muted-foreground">{action.detail}</span>}
      </div>
      {action.primary && (
        <Button
          size="sm"
          disabled={busy}
          onClick={() => onCommand(action.primary!.command, action)}
          data-testid="next-action-primary"
        >
          {action.primary.label}
        </Button>
      )}
    </div>
  )
}
