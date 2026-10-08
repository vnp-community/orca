/**
 * ReviewTurnSwitcher.tsx — FE-CV-TASK-060-08
 *
 * Three views of a review: everything, only what changed since the previous agent turn, and the
 * previous turn (read-only; diffs of past turns are not rebuilt). Comparison is an estimate from
 * coarse file fingerprints and is labelled as such.
 *
 * @module components/review-map/turns/ReviewTurnSwitcher
 */

import { useEffect, useMemo } from 'react'
import { History } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import { tn } from '../notes/notes-i18n'
import { compareTurns } from './turn-compare-model'
import type { TurnChangeLabel, TurnCompareResult } from './turn-compare-model'
import { AgentTurnVerificationLine } from './AgentTurnVerificationLine'
import type { AgentTurnVerificationViewModel } from './agent-turn-verification-view-model'
import { selectTurnPair } from './review-turn-selection'
import type { ReviewTurnViewMode } from './review-turn-selection'

export function turnChangeLabel(label: TurnChangeLabel): string {
  switch (label) {
    case 'new_in_turn':
      return tn('ReviewTurn.label.new', 'New in this turn')
    case 'changed_in_turn':
      return tn('ReviewTurn.label.changed', 'Changed in this turn')
    case 'unchanged_since':
      return tn('ReviewTurn.label.unchanged', 'Unchanged since previous turn')
    default:
      return tn('ReviewTurn.label.reverted', 'No longer changed')
  }
}

const LABEL_ORDER: TurnChangeLabel[] = ['new_in_turn', 'changed_in_turn', 'unchanged_since', 'reverted_in_turn']

export function ReviewTurnSwitcher({
  markers,
  mode,
  onModeChange,
  selectedTurnId,
  onSelectTurn,
  sentNoteCountByTurn,
  onCompare,
  saveFailed,
  onRetrySave,
  verificationByTurn
}: {
  markers: readonly ReviewTurnMarker[]
  mode: ReviewTurnViewMode
  onModeChange: (mode: ReviewTurnViewMode) => void
  selectedTurnId: string | null
  onSelectTurn: (turnId: string) => void
  sentNoteCountByTurn?: Readonly<Record<string, number>>
  /** Receives the comparison for the "since previous" view (null otherwise) to drive the overlay. */
  onCompare?: (result: TurnCompareResult | null) => void
  saveFailed?: boolean
  onRetrySave?: () => void
  /** Agent-ran / re-run lines keyed by turnId; absent while the quality gate is off. */
  verificationByTurn?: Readonly<Record<string, AgentTurnVerificationViewModel>>
}): React.JSX.Element {
  const pair = useMemo(() => selectTurnPair(markers, selectedTurnId), [markers, selectedTurnId])
  const compare = useMemo(
    () => (pair && mode === 'since-previous' ? compareTurns(pair.previous, pair.current) : null),
    [pair, mode]
  )
  const hasPrevious = pair !== null
  const shownTurnId = mode === 'previous' ? pair?.previous.turnId : (pair?.current ?? markers.at(-1))?.turnId
  const verification = shownTurnId ? verificationByTurn?.[shownTurnId] : undefined

  useEffect(() => {
    onCompare?.(compare)
  }, [compare, onCompare])

  // A mode that needs a previous turn cannot stay selected once none exists.
  useEffect(() => {
    if (!hasPrevious && mode !== 'all') {
      onModeChange('all')
    }
  }, [hasPrevious, mode, onModeChange])

  return (
    <div className="space-y-1 text-xs" role="group" aria-label={tn('ReviewTurn.groupLabel', 'Turn comparison')}>
      <div className="flex flex-wrap items-center gap-2">
        <ToggleGroup
          type="single"
          size="sm"
          value={mode}
          onValueChange={(value) => {
            if (value) {
              onModeChange(value as ReviewTurnViewMode)
            }
          }}
        >
          <ToggleGroupItem value="all">{tn('ReviewTurn.mode.all', 'All changes')}</ToggleGroupItem>
          <ToggleGroupItem value="since-previous" disabled={!hasPrevious}>
            {tn('ReviewTurn.mode.since', 'Since previous turn')}
          </ToggleGroupItem>
          <ToggleGroupItem value="previous" disabled={!hasPrevious}>
            {tn('ReviewTurn.mode.previous', 'View previous turn')}
          </ToggleGroupItem>
        </ToggleGroup>
        {markers.length > 0 ? (
          <Popover>
            <PopoverTrigger asChild>
              <Button type="button" variant="ghost" size="xs">
                <History aria-hidden />
                {tn('ReviewTurn.turns', 'Turns ({{count}})', { count: markers.length })}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-72 p-2 text-xs">
              <ul className="space-y-0.5">
                {markers.toReversed().map((m) => (
                  <li key={m.turnId}>
                    <button
                      type="button"
                      aria-current={pair?.current.turnId === m.turnId}
                      className="flex w-full items-center justify-between gap-2 rounded px-1 py-0.5 text-left hover:bg-accent"
                      onClick={() => onSelectTurn(m.turnId)}
                    >
                      <span>
                        {m.agentType ?? tn('ReviewTurn.agent', 'Agent')} · {new Date(m.endedAt).toLocaleTimeString()}
                        {m.interrupted ? ` · ${tn('ReviewTurn.interrupted', 'interrupted')}` : ''}
                      </span>
                      <span className="text-muted-foreground">
                        {tn('ReviewTurn.sentNotes', '{{count}} sent notes', {
                          count: sentNoteCountByTurn?.[m.turnId] ?? 0
                        })}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </PopoverContent>
          </Popover>
        ) : null}
      </div>

      {!hasPrevious ? (
        <p className="text-muted-foreground">{tn('ReviewTurn.noPrevious', 'No previous turn to compare yet.')}</p>
      ) : null}

      {mode === 'since-previous' && compare ? (
        <div role="status" className="space-y-0.5">
          <ul className="flex flex-wrap gap-x-3 text-muted-foreground">
            {LABEL_ORDER.map((label) => (
              <li key={label}>
                {turnChangeLabel(label)}: {compare.counts[label]}
              </li>
            ))}
          </ul>
          <p className="text-[11px] text-muted-foreground">
            {tn('ReviewTurn.estimate', 'Estimate: based on file change counts, so an edit that keeps the same counts can be missed.')}
          </p>
        </div>
      ) : null}

      {mode === 'previous' && hasPrevious ? (
        <p role="status" className="text-muted-foreground">
          {tn('ReviewTurn.readOnly', 'Read-only view of the previous turn. Its diff is not rebuilt.')}
        </p>
      ) : null}

      {verification ? <AgentTurnVerificationLine viewModel={verification} /> : null}

      {saveFailed ? (
        <p role="alert" className="flex items-center gap-2 text-destructive">
          {tn('ReviewTurn.saveFailed', 'Could not save the turn marker.')}
          {onRetrySave ? (
            <Button type="button" variant="outline" size="xs" onClick={onRetrySave}>
              {tn('ReviewTurn.retry', 'Retry')}
            </Button>
          ) : null}
        </p>
      ) : null}
    </div>
  )
}
