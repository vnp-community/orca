/**
 * SolutionOptionCompare — CR-REQ-020-03
 *
 * Cards (radio group) or comparison table over the same options.
 *
 * @module components/request/solution/SolutionOptionCompare
 */

import React, { useRef, useState } from 'react'
import { LayoutGrid, Table2 } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import type { RequestType, Solution } from '../../../../../shared/request-types'
import { SolutionImpactSection } from '../impact/SolutionImpactSection'
import { SolutionComparisonTable } from './SolutionComparisonTable'
import { SolutionOptionCard } from './SolutionOptionCard'
import { canApproveSolution } from './solution-view-model'

export type SolutionOptionCompareProps = {
  solution: Solution
  requestType: RequestType
  selectedId: string | null
  onSelect: (optionId: string) => void
  readOnly: boolean
  onRegenerate?: () => void
  regenerateDisabled?: boolean
}

export function SolutionOptionCompare({
  solution,
  requestType,
  selectedId,
  onSelect,
  readOnly,
  onRegenerate,
  regenerateDisabled = false
}: SolutionOptionCompareProps): React.JSX.Element {
  const [view, setView] = useState<'cards' | 'table'>('cards')
  const refs = useRef(new Map<string, HTMLButtonElement>())
  const options = solution.options ?? []
  const gate = canApproveSolution({
    kind: solution.kind,
    requestType,
    options: solution.options,
    chosenOptionId: selectedId ?? undefined
  })
  const needTwo = gate.reasonKey === 'needTwoOptions'
  // Roving tabindex: the selected card, else the first, is the single tab stop.
  const tabStopId = options.some((o) => o.id === selectedId) ? selectedId : options[0]?.id

  const move = (from: number, delta: number): void => {
    if (options.length === 0) {return}
    const next = options[(from + delta + options.length) % options.length]
    refs.current.get(next.id)?.focus()
    if (!readOnly) {onSelect(next.id)}
  }

  const onKeyDown = (index: number) => (e: React.KeyboardEvent<HTMLButtonElement>): void => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault()
      move(index, 1)
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault()
      move(index, -1)
    }
  }

  return (
    <div className="space-y-3" data-testid="solution-option-compare">
      {needTwo && !readOnly && (
        <div role="alert" className="flex items-center gap-2 rounded-md border border-border bg-muted p-2 text-sm">
          <span>{translate('auto.components.request.SolutionOptionCompare.needTwo', 'At least 2 options are required.')}</span>
          {onRegenerate && (
            <Button size="sm" variant="outline" disabled={regenerateDisabled} onClick={onRegenerate}>
              {translate('auto.components.request.SolutionGenerationState.regenerate', 'Regenerate')}
            </Button>
          )}
        </div>
      )}
      {options.length === 0 && (
        <p className="text-sm text-muted-foreground">
          {translate('auto.components.request.SolutionPanel.noData', 'No data')}
        </p>
      )}
      {options.length >= 2 && (
        <div className="flex justify-end">
          <Button
            size="sm"
            variant="outline"
            onClick={() => setView(view === 'cards' ? 'table' : 'cards')}
          >
            {view === 'cards' ? <Table2 className="size-4" aria-hidden /> : <LayoutGrid className="size-4" aria-hidden />}
            {view === 'cards'
              ? translate('auto.components.request.SolutionOptionCompare.compare', 'Compare')
              : translate('auto.components.request.SolutionOptionCompare.cards', 'Cards')}
          </Button>
        </div>
      )}
      {view === 'table' && options.length >= 2 ? (
        <SolutionComparisonTable options={options} />
      ) : (
        <div className="@container">
          <div
            role="radiogroup"
            aria-label={translate('auto.components.request.SolutionOptionCompare.options', 'Options')}
            className="grid grid-cols-1 gap-3 @xl:grid-cols-2 @4xl:grid-cols-3"
          >
            {options.map((option, i) => (
              <SolutionOptionCard
                key={option.id}
                option={option}
                selected={option.id === selectedId}
                chosen={solution.status === 'chosen' && option.id === solution.chosenOptionId}
                readOnly={readOnly}
                tabIndex={option.id === tabStopId ? 0 : -1}
                onSelect={() => onSelect(option.id)}
                onKeyDown={onKeyDown(i)}
                buttonRef={(el) => {
                  if (el) {refs.current.set(option.id, el)}
                  else {refs.current.delete(option.id)}
                }}
              />
            ))}
          </div>
        </div>
      )}
      <SolutionImpactSection solution={solution} selectedId={selectedId} />
    </div>
  )
}
