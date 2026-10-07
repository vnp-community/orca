/**
 * RequestStatusBadge — CR-REQ-018-05
 *
 * Renders a status badge with icon + translated label.
 * Uses design-system token classes (no hex, no Tailwind colour primitives).
 *
 * @module components/request/RequestStatusBadge
 */

import React from 'react'
import {
  SendHorizontal, ScanSearch, CircleHelp, BrainCircuit, CircleCheck,
  MessageCircleQuestion, ListTodo, ClipboardCheck, Cpu, CheckCircle2,
  Ban, Inbox, Circle
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { getRequestStatusPresentation } from './request-status-presentation'
import type { RequestStatus } from '../../../../shared/request-types'

// Map icon names to components — avoids dynamic string imports
const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  SendHorizontal, ScanSearch, CircleHelp, BrainCircuit, CircleCheck,
  MessageCircleQuestion, ListTodo, ClipboardCheck, Cpu, CheckCircle2,
  Ban, Inbox, Circle
}

const TONE_CLASSES: Record<string, string> = {
  'status-success': 'text-[color:var(--status-success)]',
  'destructive': 'text-destructive',
  'primary': 'text-primary',
  'muted-foreground': 'text-muted-foreground'
}

type Props = {
  status: RequestStatus
  size?: 'sm' | 'xs'
  className?: string
}

export function RequestStatusBadge({ status, size = 'sm', className }: Props): React.JSX.Element {
  const pres = getRequestStatusPresentation(status)
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
