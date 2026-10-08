/**
 * SolutionImpactSection — FE-REQ-TASK-036-05
 *
 * Risk card + architecture thumbnail + Option x dimension table for the option
 * the reviewer is looking at. Hides itself when the runtime has no impact channels.
 *
 * @module components/request/impact/SolutionImpactSection
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { GraphMini } from '../../graph/GraphMini'
import { useGraphLens } from '../../../hooks/useGraphLens'
import { useImpactAssessment } from '../../../hooks/useImpactAssessment'
import { ImpactFindingList } from './ImpactFindingList'
import { RiskSummaryCard } from './RiskSummaryCard'
import { SolutionDimensionTable } from './SolutionDimensionTable'
import type { Solution } from '../../../../../shared/request-types'

const T = 'auto.components.request.impact.'

type Props = { solution: Solution; selectedId: string | null }

export function SolutionImpactSection({ solution, selectedId }: Props): React.JSX.Element | null {
  const options = solution.options ?? []
  const focusId = selectedId ?? solution.chosenOptionId ?? options[0]?.id ?? ''
  const impact = useImpactAssessment({
    requestId: solution.requestId,
    subjectType: 'solution_option',
    subjectId: focusId,
    solutionId: solution.id,
    enabled: solution.kind === 'solution' && focusId !== ''
  })
  const graph = useGraphLens({
    request: { id: solution.requestId },
    lens: 'architecture',
    subjectType: 'solution_option',
    subjectId: focusId,
    enabled: impact.status === 'ready' && impact.summary !== null
  })
  // Why: unsupported runtimes keep the SOL-020 view untouched (no error styling).
  if (solution.kind !== 'solution' || impact.status === 'unsupported' || focusId === '') {return null}

  return (
    <section className="flex flex-col gap-3" data-testid="solution-impact-section" aria-label={translate(`${T}title`, 'Impact and risk')}>
      <RiskSummaryCard assessment={impact} subjectType="solution_option" subjectId={focusId} solutionId={solution.id} />
      {graph.payload ? <GraphMini payload={graph.payload} hasListEquivalent={false} /> : null}
      <SolutionDimensionTable
        options={options.map((o) => ({ id: o.id, title: o.title }))}
        comparison={impact.comparison}
        recommendedId={options.find((o) => o.raw?.recommended === true)?.id}
      />
      {impact.findings.length > 0 ? (
        <Collapsible>
          <CollapsibleTrigger className="text-xs underline-offset-2 hover:underline">{translate(`${T}findings`, 'Findings')} ({impact.findings.length})</CollapsibleTrigger>
          <CollapsibleContent className="pt-2"><ImpactFindingList findings={impact.findings} /></CollapsibleContent>
        </Collapsible>
      ) : null}
    </section>
  )
}
