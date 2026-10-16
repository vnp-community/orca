/**
 * PlanOutline — FE-REQ-TASK-037-04
 *
 * The order of work in a plan that runs on its own: steps with their task counts and estimated hours, or one flat list. It has
 * no status badges, Run buttons or phase actions — those belong to the Execution tab. A tick shows what the run already did.
 *
 * @module components/request/plan/PlanOutline
 */

import React, { useState } from 'react'
import { Check, ChevronDown, ChevronRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../../../shared/task-types'

const T = 'auto.components.request.plan.PlanOutline.'
const FINISHED = new Set(['done', 'cancelled'])

function hoursOf(tasks: OrcaTask[]): number {
  return tasks.reduce((sum, t) => sum + (t.estimatedHours ?? 0), 0)
}

function Hours({ hours }: { hours: number }): React.JSX.Element | null {
  if (hours <= 0) {
    return null
  }
  return (
    <span className="text-xs text-muted-foreground">
      {translate(`${T}hours`, '~{{hours}}h estimated', { hours: Math.round(hours * 10) / 10 })}
    </span>
  )
}

function TaskLine({ task, index }: { task: OrcaTask; index: number }): React.JSX.Element {
  const done = FINISHED.has(task.status)
  return (
    <li
      className="flex items-center gap-2 px-2 py-1 text-sm"
      data-testid={`plan-outline-task-${task.id}`}
      data-done={done}
    >
      {done ? (
        <Check
          className="size-3.5 shrink-0 text-[color:var(--status-success)]"
          aria-label={translate(`${T}done`, 'Done')}
        />
      ) : (
        <span className="w-3.5 shrink-0 text-right text-xs text-muted-foreground" aria-hidden>
          {index + 1}
        </span>
      )}
      <span className="min-w-0 flex-1 truncate">{task.title}</span>
    </li>
  )
}

function StepLine({
  phase,
  tasks,
  index,
  open: initiallyOpen
}: {
  phase: OrcaTask
  tasks: OrcaTask[]
  index: number
  open: boolean
}): React.JSX.Element {
  const [open, setOpen] = useState(initiallyOpen)
  const done = tasks.length > 0 && tasks.every((t) => FINISHED.has(t.status))
  return (
    <li className="rounded-md border border-border" data-testid={`plan-outline-step-${phase.id}`}>
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
        {done && (
          <Check
            className="size-3.5 text-[color:var(--status-success)]"
            aria-label={translate(`${T}done`, 'Done')}
          />
        )}
        <span className="min-w-0 flex-1 truncate font-medium">
          {translate(`${T}step`, '{{index}}. {{title}}', { index: index + 1, title: phase.title })}
        </span>
        <span className="text-xs text-muted-foreground">
          {translate(`${T}taskCount`, '{{count}} tasks', { count: tasks.length })}
        </span>
        <Hours hours={hoursOf(tasks)} />
      </button>
      {open && (
        <ol className="border-t border-border py-1">
          {tasks.map((t, i) => (
            <TaskLine key={t.id} task={t} index={i} />
          ))}
        </ol>
      )}
    </li>
  )
}

type Props = { tree: PlanSubtree; hasPhases: boolean }

export function PlanOutline({ tree, hasPhases }: Props): React.JSX.Element {
  if (hasPhases && tree.phases.length > 0) {
    return (
      <ol className="flex flex-col gap-2 p-3" data-testid="plan-outline">
        {tree.phases.map((phase, i) => (
          <StepLine
            key={phase.id}
            phase={phase}
            tasks={tree.tasksByPhase[phase.id] ?? []}
            index={i}
            open={i === 0}
          />
        ))}
        {tree.flatTasks.length > 0 && (
          <li className="rounded-md border border-border py-1">
            <ol>
              {tree.flatTasks.map((t, i) => (
                <TaskLine key={t.id} task={t} index={i} />
              ))}
            </ol>
          </li>
        )}
      </ol>
    )
  }
  const tasks = [...tree.phases.flatMap((p) => tree.tasksByPhase[p.id] ?? []), ...tree.flatTasks]
  return (
    <div className="flex flex-col gap-1 p-3" data-testid="plan-outline">
      <div className="flex justify-end px-2">
        <Hours hours={hoursOf(tasks)} />
      </div>
      <ol className="rounded-md border border-border py-1" data-testid="plan-outline-flat">
        {tasks.map((t, i) => (
          <TaskLine key={t.id} task={t} index={i} />
        ))}
      </ol>
    </div>
  )
}
