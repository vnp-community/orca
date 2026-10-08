/**
 * RequestStageTimeline — CR-REQ-019-03
 *
 * Renders buildStageTimeline: icon plus text per step (never colour alone).
 *
 * @module components/request/RequestStageTimeline
 */

import React from 'react'
import { Check, Circle, CircleDot, MessageCircleQuestion, MinusCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { buildStageTimeline } from './request-stage-timeline-model'
import type { StepState } from './request-stage-timeline-model'
import type { OrcaRequest } from '../../../../shared/request-types'

const STEP_ICON: Record<StepState, React.ComponentType<{ className?: string }>> = {
  done: Check,
  current: CircleDot,
  pending: Circle,
  skipped: MinusCircle
}

const STEP_TONE: Record<StepState, string> = {
  done: 'text-[color:var(--status-success)]',
  current: 'text-primary',
  pending: 'text-muted-foreground',
  skipped: 'text-destructive'
}

export function RequestStageTimeline({
  request,
  resumeStatus
}: {
  request: OrcaRequest
  /** From the open Clarification; places the waiting marker on the step that will resume. */
  resumeStatus?: string
}): React.JSX.Element | null {
  const timeline = buildStageTimeline({
    type: request.type,
    size: request.size,
    status: request.status,
    returnedFromStage: request.returnedFromStage,
    resumeStatus
  })
  if (timeline.steps.length === 0) {
    return null
  }

  return (
    <div data-testid="request-stage-timeline">
      <ol className="flex flex-wrap items-center gap-x-4 gap-y-1">
        {timeline.steps.map((step) => {
          const Icon = STEP_ICON[step.state]
          const label = translate(step.labelKey, step.id)
          return (
            <li
              key={step.id}
              data-step-state={step.state}
              aria-current={step.state === 'current' ? 'step' : undefined}
              className={cn('flex items-center gap-1 text-xs', STEP_TONE[step.state])}
            >
              <Icon className="size-3.5" aria-hidden />
              <span>{label}</span>
              <span className="sr-only">
                {translate(`auto.components.request.StageTimeline.state.${step.state}`, step.state)}
              </span>
            </li>
          )
        })}
      </ol>
      {request.status === 'awaiting_information' && (
        <p
          className="mt-1 flex items-center gap-1 text-xs text-muted-foreground"
          data-testid="timeline-awaiting-information"
        >
          <MessageCircleQuestion className="size-3.5" aria-hidden />
          {translate(
            'auto.components.request.clarification.timelineWaiting',
            'Waiting for additional information'
          )}
        </p>
      )}
      {timeline.note === 'hotfix_fast_diagnosis' && (
        <p className="mt-1 text-xs text-muted-foreground">
          {translate(
            'auto.components.request.StageTimeline.note.hotfix',
            'Hotfix: fast diagnosis, no approval before the plan.'
          )}
        </p>
      )}
    </div>
  )
}
