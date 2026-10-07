/**
 * ApprovalStatusBadge — CR-REQ-018-05
 *
 * @module components/request/ApprovalStatusBadge
 */

import React from 'react'
import { Clock, CheckCircle2, XCircle, Timer, Circle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { getApprovalPresentation } from './request-status-presentation'
import type { ApprovalStatus } from '../../../../shared/request-types'

const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  Clock, CheckCircle2, XCircle, Timer, Circle
}

const TONE_CLASSES: Record<string, string> = {
  'status-success': 'text-[color:var(--status-success)]',
  'destructive': 'text-destructive',
  'primary': 'text-primary',
  'muted-foreground': 'text-muted-foreground'
}

type Props = {
  status: ApprovalStatus
  size?: 'sm' | 'xs'
  className?: string
}

export function ApprovalStatusBadge({ status, size = 'sm', className }: Props): React.JSX.Element {
  const pres = getApprovalPresentation(status)
  const IconComponent = ICONS[pres.iconName] ?? Circle
  const toneClass = TONE_CLASSES[pres.toneClass] ?? 'text-muted-foreground'
  const label = translate(pres.labelKey, status)

  return (
    <span
      aria-label={label}
      className={cn(
        'inline-flex items-center gap-1 rounded border border-current/30 bg-current/5 px-1.5 py-0.5',
        size === 'xs' ? 'text-[10px]' : 'text-xs',
        toneClass,
        className
      )}
    >
      <IconComponent className={size === 'xs' ? 'size-2.5' : 'size-3'} aria-hidden />
      <span>{label}</span>
    </span>
  )
}
