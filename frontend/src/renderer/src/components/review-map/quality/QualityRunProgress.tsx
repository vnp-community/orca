/**
 * QualityRunProgress.tsx — FE-CV-TASK-087-06
 *
 * Progress of the active run. A known percent drives a bar; an unknown one (null) is a plain
 * spinner with the stage text, never an invented number. Honest about cancelling: it is only
 * a request until the backend reports the run finished.
 *
 * @module components/review-map/quality/QualityRunProgress
 */

import { Loader2 } from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import type { ActiveQualityRun } from '../../../store/slices/code-intel-quality-state'
import { scorecardCopy } from './quality-scorecard-copy'

export function QualityRunProgress({
  run,
  spinnerVisible
}: {
  run: ActiveQualityRun
  spinnerVisible: boolean
}): React.JSX.Element {
  const headline =
    run.phase === 'cancelling'
      ? scorecardCopy('run.cancelling')
      : run.phase === 'queued'
        ? scorecardCopy('run.queued')
        : run.phase === 'starting'
          ? scorecardCopy('run.starting')
          : scorecardCopy('run.running')
  const percent =
    run.percent !== null && Number.isFinite(run.percent)
      ? Math.min(100, Math.max(0, run.percent))
      : null
  return (
    <div
      className="flex min-w-0 flex-col gap-1"
      role="status"
      aria-live="polite"
      data-testid="quality-run-progress"
      data-phase={run.phase}
    >
      <div className="flex items-center gap-2 text-xs text-foreground">
        {spinnerVisible && percent === null ? (
          <Loader2 className="size-3.5 animate-spin motion-reduce:animate-none" aria-hidden />
        ) : null}
        <span>{headline}</span>
        {run.stage ? (
          <span className="text-muted-foreground">
            {scorecardCopy('run.stage', { stage: run.stage })}
          </span>
        ) : null}
        {run.stepIndex !== undefined && run.stepCount !== undefined ? (
          <span className="text-muted-foreground">
            {scorecardCopy('run.step', { index: run.stepIndex, count: run.stepCount })}
          </span>
        ) : null}
      </div>
      {percent !== null ? (
        <div className="flex items-center gap-2">
          <Progress value={percent} className="h-1.5" aria-label={headline} />
          <span className="text-xs tabular-nums text-muted-foreground">
            {scorecardCopy('run.percent', { percent: Math.round(percent) })}
          </span>
        </div>
      ) : (
        <span className="sr-only">{scorecardCopy('run.progressUnknown')}</span>
      )}
      {run.message ? (
        <p className="break-words text-xs text-muted-foreground">{run.message}</p>
      ) : null}
    </div>
  )
}
