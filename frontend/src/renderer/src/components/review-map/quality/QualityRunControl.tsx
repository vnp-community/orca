/**
 * QualityRunControl.tsx — FE-CV-TASK-087-06
 *
 * Choose a runnable profile and scope, start the checks, watch progress, cancel. The button
 * locks the instant it is pressed; the spinner only appears after a delay. Only profile names
 * are offered (no command input), and Cancel is a quiet ghost action, not a destructive one.
 *
 * @module components/review-map/quality/QualityRunControl
 */

import { Loader2, Play } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'
import type { ActiveQualityRun } from '../../../store/slices/code-intel-quality-state'
import { QualityRunProgress } from './QualityRunProgress'
import { MissingList } from './QualityRunNotice'
import { allowedRunScopes } from './quality-run-scope-model'
import type { QualityRunScopeChoice } from './quality-run-scope-model'
import { scorecardCopy } from './quality-scorecard-copy'
import type { QualityScorecardCopyKey } from './quality-scorecard-copy'
import { useDelayedFlag } from './use-delayed-flag'

export type QualityRunControlProps = {
  runnable: readonly RunnableProfile[]
  selected: string | null
  selectedProfile: RunnableProfile | null
  scope: QualityRunScopeChoice
  run: ActiveQualityRun | null
  /** Running is impossible right now (offline or no permission). */
  locked: boolean
  spinnerDelayMs: number
  onSelectProfile: (name: string) => void
  onSelectScope: (scope: QualityRunScopeChoice) => void
  onStart: () => void
  onCancel: () => void
}

export function QualityRunControl(props: QualityRunControlProps): React.JSX.Element {
  const { runnable, selected, selectedProfile, scope, run, locked } = props
  const active = run !== null && run.phase !== 'finished'
  const starting = run?.phase === 'starting'
  const spinnerVisible = useDelayedFlag(active, props.spinnerDelayMs)
  const notReady = selectedProfile !== null && !selectedProfile.ready
  const canStart = !active && !locked && selected !== null && !notReady
  const scopes = allowedRunScopes(selectedProfile)

  return (
    <div className="flex flex-col gap-2" data-testid="quality-run-control">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={selected ?? undefined}
          onValueChange={props.onSelectProfile}
          disabled={active || runnable.length === 0}
        >
          <SelectTrigger size="sm" className="w-48" aria-label={scorecardCopy('run.profile')}>
            <SelectValue placeholder={scorecardCopy('run.noProfiles')} />
          </SelectTrigger>
          <SelectContent>
            {runnable.map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.title || p.id}
                {p.heavy ? ` (${scorecardCopy('run.heavy')})` : ''}
                {p.ready ? '' : ` · ${scorecardCopy('run.notReady')}`}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={scope}
          onValueChange={(v) => props.onSelectScope(v as QualityRunScopeChoice)}
          disabled={active}
        >
          <SelectTrigger size="sm" className="w-40" aria-label={scorecardCopy('run.scopeLabel')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {scopes.map((s) => (
              <SelectItem key={s} value={s}>
                {scorecardCopy(`run.scope.${s}` as QualityScorecardCopyKey)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {/* Fixed-width label slot so the button does not jump between Run and Starting. */}
        <Button
          type="button"
          size="sm"
          className="min-w-28"
          disabled={!canStart}
          aria-busy={starting}
          onClick={props.onStart}
        >
          {starting && spinnerVisible ? (
            <Loader2 className="animate-spin motion-reduce:animate-none" aria-hidden />
          ) : (
            <Play aria-hidden />
          )}
          {starting ? scorecardCopy('run.starting') : scorecardCopy('run.start')}
        </Button>
        {active && !starting ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={run?.phase === 'cancelling'}
            onClick={props.onCancel}
          >
            {scorecardCopy('run.cancel')}
          </Button>
        ) : null}
      </div>
      {notReady ? <MissingList missing={selectedProfile?.missing ?? []} /> : null}
      {run && active ? <QualityRunProgress run={run} spinnerVisible={spinnerVisible} /> : null}
    </div>
  )
}
