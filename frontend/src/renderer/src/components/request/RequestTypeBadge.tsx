/**
 * RequestTypeBadge — CR-REQ-018-05
 *
 * Renders a request type badge. Shows "Chưa phân loại" label when type is unknown.
 *
 * @module components/request/RequestTypeBadge
 */

import React from 'react'
import {
  Bug, CheckSquare, BookOpen, MessageCircleQuestion, Zap,
  ShieldAlert, Server, GitPullRequest, RefreshCw, FlaskConical,
  Gauge, Circle
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { getRequestTypePresentation } from './request-status-presentation'
import type { RequestType } from '../../../../shared/request-types'

const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  Bug, CheckSquare, BookOpen, MessageCircleQuestion, Zap,
  ShieldAlert, Server, GitPullRequest, RefreshCw, FlaskConical,
  Gauge, Circle
}

const TONE_CLASSES: Record<string, string> = {
  'status-success': 'text-[color:var(--status-success)]',
  'destructive': 'text-destructive',
  'primary': 'text-primary',
  'muted-foreground': 'text-muted-foreground'
}

type Props = {
  type: RequestType | undefined | null
  size?: 'sm' | 'xs'
  className?: string
}

export function RequestTypeBadge({ type, size = 'sm', className }: Props): React.JSX.Element {
  const safeType: RequestType = !type || type === 'unknown' ? 'unknown' : type
  const pres = getRequestTypePresentation(safeType)
  const IconComponent = ICONS[pres.iconName] ?? Circle
  const toneClass = TONE_CLASSES[pres.toneClass] ?? 'text-muted-foreground'
  const label = translate(pres.labelKey, safeType === 'unknown' ? 'Unclassified' : safeType)

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
