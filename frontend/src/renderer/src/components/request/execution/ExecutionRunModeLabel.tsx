/**
 * ExecutionRunModeLabel — FE-REQ-TASK-043-04
 *
 * A quiet chip that says whether the run goes on by itself or asks before each step, so nobody wonders why approvals keep
 * coming. Display only: changing the mode of a running Request is a different feature.
 *
 * @module components/request/execution/ExecutionRunModeLabel
 */

import React from 'react'
import { FastForward, Hand } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import type { RunModeLabel } from './execution-run-mode-model'

const T = 'auto.components.request.execution.ExecutionRunModeLabel.'

export function ExecutionRunModeLabel({ mode }: { mode: RunModeLabel }): React.JSX.Element {
  const continuous = mode === 'continuous'
  const Icon = continuous ? FastForward : Hand
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          {/* Why: focusable so the tooltip is reachable without a mouse. */}
          <Badge variant="outline" tabIndex={0} data-testid="execution-run-mode" data-mode={mode}>
            <Icon aria-hidden />
            {continuous
              ? translate(`${T}continuous`, 'Runs all steps on its own')
              : translate(`${T}stepByStep`, 'Asks before each step')}
          </Badge>
        </TooltipTrigger>
        <TooltipContent>
          {translate(
            `${T}tooltip`,
            'Steps run with the access of the person who approved the plan.'
          )}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
