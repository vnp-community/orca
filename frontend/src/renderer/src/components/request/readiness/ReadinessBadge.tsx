/**
 * ReadinessBadge — FE-REQ-TASK-036-07
 *
 * Text + icon; unchecked (null/unknown) tasks show nothing. Work tasks only.
 *
 * @module components/request/readiness/ReadinessBadge
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { getReadinessPresentation } from './readiness-action-rules'
import type { TaskReadinessReport } from '../../../../../shared/request-artifact-types'

const TONE: Record<string, string> = {
  success: 'text-status-success border-status-success-border bg-status-success-background',
  neutral: 'text-muted-foreground border-border',
  warning: 'text-risk-medium border-risk-medium-border bg-risk-medium-background',
  destructive: 'text-destructive border-destructive/40'
}

type Props = { report: TaskReadinessReport | null; compact?: boolean; onOpen?: () => void; className?: string }

export function ReadinessBadge({ report, compact, onOpen, className }: Props): React.JSX.Element | null {
  const pres = report ? getReadinessPresentation(report.outcome) : null
  if (!report || !pres) {return null}
  const label = translate(pres.labelKey, pres.fallback)
  const tip = [report.tier, report.checkedAt].filter(Boolean).join(' · ')
  const badge = (
    <Badge
      variant="outline"
      data-readiness={report.outcome}
      className={cn(TONE[pres.tone], !compact && onOpen && 'cursor-pointer', compact && 'px-1.5 text-[10px]', className)}
      onClick={!compact && onOpen ? onOpen : undefined}
    >
      <pres.Icon aria-hidden />
      <span>{label}</span>
    </Badge>
  )
  if (!tip) {return badge}
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{badge}</TooltipTrigger>
        <TooltipContent>{tip}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
