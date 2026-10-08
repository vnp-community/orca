/**
 * BacklogGateStatusBadge — CR-REQ-023-06
 *
 * Plan/Phase gate state; icon + label so colour is never the only signal.
 *
 * @module components/request/backlog/BacklogGateStatusBadge
 */

import React from 'react'
import { CheckCircle2, CircleDashed, CircleHelp, Hourglass, XCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { GateStatus } from '../../../../../shared/request-backlog-types'

const TONES: Record<GateStatus, { icon: React.ComponentType<{ className?: string }>; tone: string; label: string }> = {
  approved: { icon: CheckCircle2, tone: 'text-[color:var(--status-success)]', label: 'Approved' },
  pending: { icon: Hourglass, tone: 'text-primary', label: 'Awaiting approval' },
  rejected: { icon: XCircle, tone: 'text-destructive', label: 'Rejected' },
  none: { icon: CircleDashed, tone: 'text-muted-foreground', label: 'No gate' },
  unknown: { icon: CircleHelp, tone: 'text-muted-foreground', label: 'Unknown' }
}

export function BacklogGateStatusBadge({ status }: { status: GateStatus }): React.JSX.Element {
  const { icon: Icon, tone, label } = TONES[status]
  return (
    <span
      className={cn('inline-flex items-center gap-1 rounded border border-current/30 bg-current/5 px-1.5 py-0.5 text-xs', tone)}
      data-testid="gate-status-badge"
    >
      <Icon className="size-3" aria-hidden />
      {translate(`auto.components.request.backlog.GateStatus.${status}`, label)}
    </span>
  )
}
