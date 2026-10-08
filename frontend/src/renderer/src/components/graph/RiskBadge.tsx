/**
 * RiskBadge — FE-REQ-TASK-032-01
 *
 * Label + icon + token style. Never says "safe"; unassessed is its own state.
 *
 * @module components/graph/RiskBadge
 */

import React from 'react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { riskPresentation } from './risk-presentation'
import type { GraphRisk } from '../../../../shared/graph-types'

type Props = {
  level: GraphRisk
  assessedAt?: string | null
  tool?: string | null
  size?: 'sm' | 'md'
  showLabel?: boolean
  className?: string
}

export function RiskBadge({
  level,
  assessedAt,
  tool,
  size = 'md',
  showLabel = true,
  className
}: Props): React.JSX.Element {
  const pres = riskPresentation(level)
  const label = translate(pres.labelKey, pres.labelFallback)
  const tooltip = assessedAt
    ? translate(
        'auto.components.graph.RiskTooltip',
        'Risk {{level}}, assessed {{assessedAt}}, based on {{tool}}',
        { level: label, assessedAt, tool: tool ?? '-' }
      )
    : translate('auto.components.graph.RiskTooltipUnassessed', 'Risk {{level}}, no assessment time recorded', {
        level: label
      })
  const Icon = pres.Icon

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge
            variant="outline"
            data-risk-level={pres.level}
            aria-label={showLabel ? undefined : label}
            className={cn(
              pres.textClass,
              pres.bgClass,
              pres.borderClass,
              pres.borderWidthClass,
              pres.dashed && 'border-dashed',
              pres.doubleRing && 'ring-2 ring-risk-critical-border',
              size === 'sm' ? 'px-1.5 text-[10px]' : 'text-xs',
              className
            )}
          >
            <Icon aria-hidden />
            {showLabel ? <span>{label}</span> : null}
          </Badge>
        </TooltipTrigger>
        <TooltipContent>{tooltip}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
