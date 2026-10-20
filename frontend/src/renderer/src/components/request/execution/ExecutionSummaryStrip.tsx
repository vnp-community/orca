/**
 * ExecutionSummaryStrip — FE-REQ-TASK-037-05
 *
 * Where the run is at a glance: progress bar, "27/28 tasks · step 7/7", how long it has been going and a state label with an
 * icon. The only control is "Stop run" (cancel the Request, with a confirmation owned by the pane); there is no Pause (F12).
 *
 * @module components/request/execution/ExecutionSummaryStrip
 */

import React from 'react'
import { Check, Hand, Loader2, OctagonX } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { ExecutionRunModeLabel } from './ExecutionRunModeLabel'
import type { RunModeLabel } from './execution-run-mode-model'
import type { ExecutionSummary } from '../../../../../shared/request-execution-types'
import type { RequestStatus } from '../../../../../shared/request-types'

const T = 'auto.components.request.execution.ExecutionSummaryStrip.'

export type RunLabel = 'running' | 'needs_you' | 'stopped' | 'done'

export function runLabelOf(status: RequestStatus, summary: ExecutionSummary): RunLabel {
  if (status === 'completed') {
    return 'done'
  }
  if (status === 'request_backlog' || status === 'cancelled') {
    return 'stopped'
  }
  return summary.needsYou.length > 0 || status === 'awaiting_information' ? 'needs_you' : 'running'
}

const LABEL_ICON: Record<RunLabel, React.ComponentType<{ className?: string }>> = {
  running: Loader2,
  needs_you: Hand,
  stopped: OctagonX,
  done: Check
}

// Why: "running" and "needs you" share the primary tone on purpose (no new colour token); icon and text tell them apart.
const LABEL_TONE: Record<RunLabel, string> = {
  running: 'text-primary',
  needs_you: 'text-primary',
  stopped: 'text-destructive',
  done: 'text-[color:var(--status-success)]'
}

function labelText(label: RunLabel): string {
  switch (label) {
    case 'running':
      return translate(`${T}running`, 'Running')
    case 'needs_you':
      return translate(`${T}needsYou`, 'Needs you')
    case 'stopped':
      return translate(`${T}stopped`, 'Stopped')
    default:
      return translate(`${T}done`, 'Done')
  }
}

/** "12m", "1h 05m", "2d 3h"; null for a missing or future start. */
export function formatElapsed(startedAt: string | undefined, now: number): string | null {
  const start = startedAt ? Date.parse(startedAt) : Number.NaN
  if (Number.isNaN(start) || start > now) {
    return null
  }
  const minutes = Math.floor((now - start) / 60_000)
  if (minutes < 1) {
    return '<1m'
  }
  if (minutes < 60) {
    return `${minutes}m`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours}h ${String(minutes % 60).padStart(2, '0')}m`
  }
  return `${Math.floor(hours / 24)}d ${hours % 24}h`
}

type Props = {
  status: RequestStatus
  summary: ExecutionSummary
  /** Offered while the run can still be stopped. */
  onStop?: () => void
  /** How the run goes on (CR-REQ-043); null or omitted shows nothing. */
  runMode?: RunModeLabel | null
  now?: number
}

export function ExecutionSummaryStrip({
  status,
  summary,
  onStop,
  runMode = null,
  now = Date.now()
}: Props): React.JSX.Element {
  const { progress } = summary
  const label = runLabelOf(status, summary)
  const Icon = LABEL_ICON[label]
  const percent =
    progress.tasksTotal > 0 ? Math.round((progress.tasksDone / progress.tasksTotal) * 100) : 0
  const elapsed = label === 'done' ? null : formatElapsed(summary.startedAt, now)
  const stoppable = onStop && (label === 'running' || label === 'needs_you')
  return (
    <div
      className="flex flex-col gap-2 rounded-md border border-border px-3 py-2"
      data-testid="execution-summary-strip"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
        <span
          className={cn('flex items-center gap-1 font-medium', LABEL_TONE[label])}
          data-testid="execution-run-label"
        >
          <Icon className={cn('size-4', label === 'running' && 'animate-spin')} aria-hidden />
          {labelText(label)}
        </span>
        {runMode && <ExecutionRunModeLabel mode={runMode} />}
        <span data-testid="execution-progress-text">
          {progress.phasesTotal > 0
            ? translate(`${T}progressSteps`, '{{done}}/{{total}} tasks · step {{step}}/{{steps}}', {
                done: progress.tasksDone,
                total: progress.tasksTotal,
                step: Math.min(progress.phasesDone + 1, progress.phasesTotal),
                steps: progress.phasesTotal
              })
            : translate(`${T}progress`, '{{done}}/{{total}} tasks', {
                done: progress.tasksDone,
                total: progress.tasksTotal
              })}
        </span>
        {elapsed && (
          <span className="text-muted-foreground">
            {translate(`${T}elapsed`, 'running for {{time}}', { time: elapsed })}
          </span>
        )}
        {stoppable && (
          <Button
            size="xs"
            variant="outline"
            className="ml-auto"
            onClick={onStop}
            data-testid="execution-stop-run"
          >
            {translate(`${T}stop`, 'Stop run')}
          </Button>
        )}
      </div>
      <Progress
        value={percent}
        className="h-1.5"
        aria-label={translate(`${T}progressLabel`, 'Run progress')}
      />
    </div>
  )
}
