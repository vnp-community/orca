/**
 * ExecutionStepList — FE-REQ-TASK-037-05
 *
 * The steps of the plan as a run goes through them: finished steps fold into one line, the step the run is at is open, later
 * steps are only counted. A plan without steps is one flat list.
 *
 * @module components/request/execution/ExecutionStepList
 */

import React, { useState } from 'react'
import { Check, ChevronDown, ChevronRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'
import { ExecutionTaskRow } from './ExecutionTaskRow'
import { deriveTaskDisplayState, type TaskDisplay } from './execution-task-state'
import { previousOpenTask, type ExecutionStep } from './execution-step-model'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { OrcaTask } from '../../../../../shared/task-types'

const T = 'auto.components.request.execution.ExecutionStepList.'
// Why: every open log subscribes to an agent stream; parallel runs can be many, so only the first few follow live.
const MAX_LIVE_LOGS = 2

type Props = {
  steps: ExecutionStep[]
  summary: ExecutionSummary | null
  onOpenTask: (task: OrcaTask) => void
}

function StepBody({
  step,
  summary,
  liveBudget,
  onOpenTask
}: {
  step: ExecutionStep
  summary: ExecutionSummary | null
  liveBudget: { left: number }
  onOpenTask: (task: OrcaTask) => void
}): React.JSX.Element {
  return (
    <ul className="flex flex-col">
      {step.tasks.map((task, i) => {
        const display: TaskDisplay = deriveTaskDisplayState(task, summary, {
          inCurrentStep: step.state === 'current',
          previousOpen: previousOpenTask(step.tasks, i)
        })
        let showLog = false
        if (display.state === 'running' && liveBudget.left > 0) {
          liveBudget.left--
          showLog = true
        }
        return (
          <ExecutionTaskRow
            key={task.id}
            task={task}
            display={display}
            showLog={showLog}
            onOpen={onOpenTask}
          />
        )
      })}
    </ul>
  )
}

function FoldedStep({
  step,
  summary,
  liveBudget,
  onOpenTask,
  index,
  total
}: {
  step: ExecutionStep
  summary: ExecutionSummary | null
  liveBudget: { left: number }
  onOpenTask: (task: OrcaTask) => void
  index: number
  total: number
}): React.JSX.Element {
  const [open, setOpen] = useState(false)
  return (
    <li
      className="rounded-md border border-border"
      data-testid={`execution-step-${step.id}`}
      data-step-state="done"
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {open ? (
          <ChevronDown className="size-3.5" aria-hidden />
        ) : (
          <ChevronRight className="size-3.5" aria-hidden />
        )}
        <Check className="size-3.5 text-[color:var(--status-success)]" aria-hidden />
        <span className="min-w-0 flex-1 truncate">
          {translate(`${T}stepTitle`, 'Step {{index}}/{{total}} · {{title}}', {
            index,
            total,
            title: step.title
          })}
        </span>
        <span className="text-xs text-muted-foreground">
          {translate(`${T}tasksDone`, '{{done}}/{{total}} tasks done', {
            done: step.done,
            total: step.tasks.length
          })}
        </span>
      </button>
      {open && (
        <div className="border-t border-border px-1 py-1">
          <StepBody step={step} summary={summary} liveBudget={liveBudget} onOpenTask={onOpenTask} />
        </div>
      )}
    </li>
  )
}

export function ExecutionStepList({ steps, summary, onOpenTask }: Props): React.JSX.Element {
  const liveBudget = { left: MAX_LIVE_LOGS }
  const named = steps.filter((s) => s.id !== null)
  if (named.length === 0) {
    const flat = steps[0]
    return flat ? (
      <div className="rounded-md border border-border p-1" data-testid="execution-flat-tasks">
        <StepBody
          step={{ ...flat, state: 'current' }}
          summary={summary}
          liveBudget={liveBudget}
          onOpenTask={onOpenTask}
        />
      </div>
    ) : (
      <p className="text-sm text-muted-foreground">
        {translate(`${T}empty`, 'This plan has no tasks yet.')}
      </p>
    )
  }
  const later = steps.filter((s) => s.state === 'later')
  return (
    <ol className="flex flex-col gap-2" data-testid="execution-step-list">
      {steps.map((step) => {
        const position = named.findIndex((s) => s.id === step.id) + 1
        if (step.state === 'done') {
          return (
            <FoldedStep
              key={step.id ?? 'flat'}
              step={step}
              summary={summary}
              liveBudget={liveBudget}
              onOpenTask={onOpenTask}
              index={position}
              total={named.length}
            />
          )
        }
        if (step.state === 'later') {
          return null
        }
        return (
          <li
            key={step.id ?? 'flat'}
            className={cn('rounded-md border border-primary/40')}
            data-testid={`execution-step-${step.id}`}
            data-step-state="current"
          >
            <div className="flex items-center gap-2 px-3 py-2 text-sm font-medium">
              <span className="min-w-0 flex-1 truncate">
                {translate(`${T}stepTitle`, 'Step {{index}}/{{total}} · {{title}}', {
                  index: position,
                  total: named.length,
                  title: step.title
                })}
              </span>
              <span className="text-xs font-normal text-muted-foreground">
                {translate(`${T}tasksDone`, '{{done}}/{{total}} tasks done', {
                  done: step.done,
                  total: step.tasks.length
                })}
              </span>
            </div>
            <div className="border-t border-border px-1 py-1">
              <StepBody
                step={step}
                summary={summary}
                liveBudget={liveBudget}
                onOpenTask={onOpenTask}
              />
            </div>
          </li>
        )
      })}
      {later.length > 0 && (
        <li className="px-3 text-xs text-muted-foreground" data-testid="execution-later-steps">
          {translate(`${T}later`, '{{count}} more steps · {{tasks}} tasks to go', {
            count: later.length,
            tasks: later.reduce((n, s) => n + s.tasks.length, 0)
          })}
        </li>
      )}
    </ol>
  )
}
