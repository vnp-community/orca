/**
 * SolutionImpactSection — FE-REQ-TASK-036-05
 *
 * Risk card + architecture thumbnail + Option x dimension table for the option
 * the reviewer is looking at. Hides itself when the runtime has no impact channels.
 *
 * @module components/request/impact/SolutionImpactSection
 */

import React, { useState } from 'react'
import { Network } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { GraphMini } from '../../graph/GraphMini'
import { RequestGraphSheet } from '../../graph/RequestGraphSheet'
import { useOpenDevServerSettings } from '../use-open-dev-server-settings'
import { useAppStore } from '@/store'
import { useGraphLens } from '../../../hooks/useGraphLens'
import { useImpactAssessment } from '../../../hooks/useImpactAssessment'
import { ImpactFindingList } from './ImpactFindingList'
import { RiskSummaryCard } from './RiskSummaryCard'
import { SolutionDimensionTable } from './SolutionDimensionTable'
import type { Solution, SolutionOption } from '../../../../../shared/request-types'

const T = 'auto.components.request.impact.'

type Props = { solution: Solution; selectedId: string | null }

/** SOL-020 rows that sit under the 9 impact dimensions; rollback has no typed field yet. */
function rollbackOf(o: SolutionOption): string | undefined {
  const raw = o.raw ?? {}
  const v = raw.rollback ?? raw.rollbackPlan ?? raw.rollback_plan
  return typeof v === 'string' && v.trim() !== '' ? v : undefined
}

export function SolutionImpactSection({ solution, selectedId }: Props): React.JSX.Element | null {
  const options = solution.options ?? []
  const openDevServerSettings = useOpenDevServerSettings()
  const [graph, setGraph] = useState<{ selectedId?: string } | null>(null)
  // Why: GraphPanel needs the full Request (flow/plan lenses); useRequest caches it in the store.
  const request = useAppStore((s) => s.requestsById[solution.requestId])
  const focusId = selectedId ?? solution.chosenOptionId ?? options[0]?.id ?? ''
  const impact = useImpactAssessment({
    requestId: solution.requestId,
    subjectType: 'solution_option',
    subjectId: focusId,
    solutionId: solution.id,
    enabled: solution.kind === 'solution' && focusId !== ''
  })
  const mini = useGraphLens({
    request: { id: solution.requestId },
    lens: 'architecture',
    subjectType: 'solution_option',
    subjectId: focusId,
    enabled: impact.status === 'ready' && impact.summary !== null
  })
  // Why: unsupported runtimes keep the SOL-020 view untouched (no error styling).
  if (solution.kind !== 'solution' || impact.status === 'unsupported' || focusId === '') {
    return null
  }

  return (
    <section
      className="flex flex-col gap-3"
      data-testid="solution-impact-section"
      aria-label={translate(`${T}title`, 'Impact and risk')}
    >
      <RiskSummaryCard
        assessment={impact}
        subjectType="solution_option"
        subjectId={focusId}
        solutionId={solution.id}
        onConnectDevServer={openDevServerSettings}
      />
      {mini.payload ? <GraphMini payload={mini.payload} hasListEquivalent={false} /> : null}
      {request ? (
        <Button size="xs" variant="outline" className="self-start" onClick={() => setGraph({})}>
          <Network className="size-3" aria-hidden />
          {translate('auto.components.graph.Entry.viewGraph', 'View graph')}
        </Button>
      ) : null}
      <SolutionDimensionTable
        options={options.map((o) => ({
          id: o.id,
          title: o.title,
          effort: o.estimatedEffort,
          rollback: rollbackOf(o)
        }))}
        comparison={impact.comparison}
        recommendedId={options.find((o) => o.raw?.recommended === true)?.id}
      />
      {impact.findings.length > 0 ? (
        <Collapsible>
          <CollapsibleTrigger className="text-xs underline-offset-2 hover:underline">
            {translate(`${T}findings`, 'Findings')} ({impact.findings.length})
          </CollapsibleTrigger>
          <CollapsibleContent className="pt-2">
            <ImpactFindingList
              findings={impact.findings}
              onViewOnGraph={request ? (f) => setGraph({ selectedId: f.nodeIds?.[0] }) : undefined}
            />
          </CollapsibleContent>
        </Collapsible>
      ) : null}
      {graph && request ? (
        <RequestGraphSheet
          open
          onOpenChange={(open) => {
            if (!open) {
              setGraph(null)
            }
          }}
          request={request}
          subject={{ type: 'solution_option', id: focusId }}
          lensInitial={graph.selectedId ? 'impact' : 'architecture'}
          impactAssessed={impact.summary !== null}
          initialSelectedId={graph.selectedId}
        />
      ) : null}
    </section>
  )
}
