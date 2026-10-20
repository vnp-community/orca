/**
 * RequestStageTimeline — CR-REQ-019-03, redesigned by CR-REQ-037 2.3
 *
 * Renders buildStageTimeline: an icon and text per stage (never colour alone), a short caption under the stage the request is
 * at ("step 3/7 · 9/28 tasks", "waiting for approval"), and a line for a request that waits for information.
 *
 * @module components/request/RequestStageTimeline
 */

import React from 'react'
import { Check, Circle, Hand, Loader2, MessageCircleQuestion, OctagonX } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { buildStageTimeline } from './request-stage-timeline-model'
import { reviewCaptionText } from './outcome-review/outcome-review-timeline-copy'
import type { ReviewTimelineInput } from './request-stage-timeline-review'
import type { StepCaption, StepState } from './request-stage-timeline-model'
import type { ExecutionSummary, RequestFlowView } from '../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../shared/request-types'

const STEP_ICON: Record<StepState, React.ComponentType<{ className?: string }>> = {
  done: Check,
  running: Loader2,
  waiting: Hand,
  stopped: OctagonX,
  pending: Circle
}

// Why: "running" and "waiting for you" share the primary tone on purpose (no new colour token); the icon and the words tell them apart.
const STEP_TONE: Record<StepState, string> = {
  done: 'text-[color:var(--status-success)]',
  running: 'text-primary',
  waiting: 'text-primary',
  stopped: 'text-destructive',
  pending: 'text-muted-foreground'
}

const C = 'auto.components.request.StageTimeline.caption.'

export function captionText(caption: StepCaption): string {
  switch (caption.kind) {
    case 'progress':
      return caption.phases
        ? translate(`${C}progressPhases`, 'step {{phase}}/{{phases}} · {{done}}/{{total}} tasks', {
            phase: caption.phase ?? 1,
            phases: caption.phases,
            done: caption.tasksDone,
            total: caption.tasksTotal
          })
        : translate(`${C}progress`, '{{done}}/{{total}} tasks', {
            done: caption.tasksDone,
            total: caption.tasksTotal
          })
    case 'stopped_progress':
      return translate(`${C}stoppedProgress`, '{{done}}/{{total}} tasks · stopped', {
        done: caption.tasksDone,
        total: caption.tasksTotal
      })
    case 'waiting_approval':
      return translate(`${C}waitingApproval`, 'waiting for approval')
    case 'waiting_you':
      return translate(`${C}waitingYou`, 'waiting for you')
    case 'archiving':
      return translate(`${C}archiving`, 'archiving')
    case 'handover_failed':
      return translate(`${C}handoverFailed`, 'archive failed')
    case 'review':
      return reviewCaptionText(caption.row, caption.openDecisions ?? 0)
  }
}

export function RequestStageTimeline({
  request,
  flow,
  summary,
  resumeStatus,
  review
}: {
  request: OrcaRequest
  /** The flow of the request's type; the built-in table answers when absent. */
  flow?: RequestFlowView | null
  summary?: ExecutionSummary | null
  /** From the open Clarification; places the waiting marker on the step that will resume. */
  resumeStatus?: string
  /** The Outcome Review row: shown only when the project has the review on. */
  review?: ReviewTimelineInput
}): React.JSX.Element | null {
  const timeline = buildStageTimeline({
    type: request.type,
    size: request.size,
    status: request.status,
    returnedFromStage: request.returnedFromStage,
    resumeStatus,
    flow,
    engine: request.engine,
    summary,
    review
  })
  if (timeline.steps.length === 0) {
    return null
  }

  return (
    <div data-testid="request-stage-timeline">
      <ol className="flex flex-wrap items-start gap-x-5 gap-y-1">
        {timeline.steps.map((step) => {
          const Icon = STEP_ICON[step.state]
          const label = translate(step.labelKey, step.id)
          return (
            <li
              key={step.id}
              data-step-id={step.id}
              data-step-state={step.state}
              aria-current={step.id === timeline.current ? 'step' : undefined}
              className={cn('flex flex-col text-xs', STEP_TONE[step.state])}
            >
              <span className="flex items-center gap-1">
                <Icon
                  className={cn('size-3.5', step.state === 'running' && 'animate-spin')}
                  aria-hidden
                />
                <span className={cn(step.id === timeline.current && 'font-medium')}>{label}</span>
                <span className="sr-only">
                  {translate(
                    `auto.components.request.StageTimeline.state.${step.state}`,
                    step.state
                  )}
                </span>
              </span>
              {step.caption && (
                <span
                  className="pl-[18px] text-[11px] text-muted-foreground"
                  data-testid={`stage-caption-${step.id}`}
                >
                  {captionText(step.caption)}
                </span>
              )}
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
    </div>
  )
}
