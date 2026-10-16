/**
 * ExecutionTaskRow — FE-REQ-TASK-037-05
 *
 * One task of the run in one of six states. Icon and words say the state (never colour alone); a running task shows the tail
 * of what its agent is doing, a failed one says how many attempts went and why.
 *
 * @module components/request/execution/ExecutionTaskRow
 */

import React from 'react'
import { Check, CircleDashed, Clock, Loader2, OctagonX, Ban } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { formatRequestRelativeTime } from '@/lib/request-relative-time'
import { TaskAgentActivityPanel } from '../../task/agent-activity/TaskAgentActivityPanel'
import { failureCauseAdvice, failureCauseLabel, shortError } from './execution-failure-cause'
import type { TaskDisplay, TaskDisplayState } from './execution-task-state'
import type { OrcaTask } from '../../../../../shared/task-types'

const T = 'auto.components.request.execution.ExecutionTaskRow.'

const ICON: Record<TaskDisplayState, React.ComponentType<{ className?: string }>> = {
  done: Check,
  running: Loader2,
  failed: OctagonX,
  blocked: Ban,
  waiting: Clock,
  queued: CircleDashed
}

const TONE: Record<TaskDisplayState, string> = {
  done: 'text-[color:var(--status-success)]',
  running: 'text-primary',
  failed: 'text-destructive',
  blocked: 'text-destructive',
  waiting: 'text-muted-foreground',
  queued: 'text-muted-foreground'
}

export function stateText(display: TaskDisplay): string {
  switch (display.state) {
    case 'done':
      return translate(`${T}done`, 'Done')
    case 'running':
      return display.attempt && display.maxAttempts
        ? translate(`${T}runningAttempt`, 'Running · attempt {{attempt}} of {{max}}', {
            attempt: display.attempt,
            max: display.maxAttempts
          })
        : translate(`${T}running`, 'Running')
    case 'failed':
      return display.attempt && display.maxAttempts
        ? translate(`${T}failedAttempts`, 'Failed {{attempt}} of {{max}}', {
            attempt: display.attempt,
            max: display.maxAttempts
          })
        : translate(`${T}failed`, 'Failed')
    case 'blocked':
      return translate(`${T}blocked`, 'Blocked')
    case 'waiting':
      return translate(`${T}waiting`, 'Waiting for “{{title}}”', { title: display.waitsOn ?? '' })
    default:
      return translate(`${T}queued`, 'Queued')
  }
}

type Props = {
  task: OrcaTask
  display: TaskDisplay
  /** Show the live agent log under a running task. */
  showLog: boolean
  onOpen: (task: OrcaTask) => void
}

export function ExecutionTaskRow({ task, display, showLog, onOpen }: Props): React.JSX.Element {
  const Icon = ICON[display.state]
  const error = display.state === 'failed' ? shortError(display.lastError) : undefined
  return (
    <li
      className="flex flex-col gap-1 rounded px-2 py-1.5"
      data-testid={`execution-task-${task.id}`}
      data-task-state={display.state}
    >
      <div className="flex items-center gap-2 text-sm">
        <Icon
          className={cn(
            'size-3.5 shrink-0',
            TONE[display.state],
            display.state === 'running' && 'animate-spin'
          )}
          aria-hidden
        />
        {task.taskNumber !== undefined && (
          <span className="font-mono text-xs text-muted-foreground">#TG-{task.taskNumber}</span>
        )}
        <span className="min-w-0 flex-1 truncate">{task.title}</span>
        <span
          className={cn('text-xs', TONE[display.state])}
          data-testid={`execution-task-state-${task.id}`}
        >
          {stateText(display)}
        </span>
        <Button
          size="xs"
          variant="ghost"
          onClick={() => onOpen(task)}
          data-testid={`execution-task-open-${task.id}`}
        >
          {display.state === 'running' || display.state === 'failed'
            ? translate(`${T}log`, 'Log')
            : translate(`${T}details`, 'Details')}
        </Button>
      </div>
      {display.state === 'failed' && (
        <div
          className="ml-6 flex flex-col gap-0.5 text-xs"
          data-testid={`execution-task-failure-${task.id}`}
        >
          <span className="text-foreground">
            {display.cause
              ? failureCauseLabel(display.cause)
              : translate(`${T}noCause`, 'No cause recorded')}
            {display.lastAt ? ` · ${formatRequestRelativeTime(display.lastAt)}` : ''}
          </span>
          {error && <span className="break-words text-muted-foreground">{error}</span>}
          {display.cause && (
            <span className="text-muted-foreground">{failureCauseAdvice(display.cause)}</span>
          )}
        </div>
      )}
      {display.state === 'running' && showLog && (
        <div className="ml-6" data-testid={`execution-task-log-${task.id}`}>
          <TaskAgentActivityPanel task={task} isRunning />
        </div>
      )}
    </li>
  )
}
