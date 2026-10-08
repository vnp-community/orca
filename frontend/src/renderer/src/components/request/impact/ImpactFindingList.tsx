/**
 * ImpactFindingList — FE-REQ-TASK-036-05
 *
 * @module components/request/impact/ImpactFindingList
 */

import React, { useMemo, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { RiskBadge } from '../../graph/RiskBadge'
import { ImpactEvidenceSheet } from './ImpactEvidenceSheet'
import { sortByRiskDesc } from './impact-dimension-model'
import type { ImpactFinding } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.impact.'

type Props = {
  findings: readonly ImpactFinding[]
  /** Opens the graph at the finding's nodes; shown only for findings that have node ids. */
  onViewOnGraph?: (finding: ImpactFinding) => void
}

export function ImpactFindingList({ findings, onViewOnGraph }: Props): React.JSX.Element {
  const [evidenceId, setEvidenceId] = useState<string | null>(null)
  const groups = useMemo(() => {
    const byDimension = new Map<string, ImpactFinding[]>()
    for (const f of findings) {byDimension.set(f.dimension, [...(byDimension.get(f.dimension) ?? []), f])}
    return [...byDimension.entries()].map(([dimension, list]) => ({ dimension, list: sortByRiskDesc(list) }))
  }, [findings])

  if (findings.length === 0) {
    return <p className="text-xs text-muted-foreground">{translate(`${T}noFindings`, 'No findings at medium level or above')}</p>
  }
  return (
    <div className="flex flex-col gap-3" data-testid="impact-finding-list">
      {groups.map((g) => (
        <section key={g.dimension} aria-label={translate(`${T}dim.${g.dimension}`, g.dimension)}>
          <h4 className="mb-1 text-xs font-medium">{translate(`${T}dim.${g.dimension}`, g.dimension)}</h4>
          <ul className="flex flex-col gap-1.5">
            {g.list.map((f) => (
              <li key={f.id} className="flex flex-wrap items-center gap-2 rounded-md border border-border px-2 py-1.5 text-xs">
                <RiskBadge level={f.level} size="sm" />
                <span className="min-w-0 flex-1 break-words">{f.title}</span>
                <Button size="xs" variant="outline" onClick={() => setEvidenceId(f.id)}>{translate(`${T}evidence`, 'Evidence')}</Button>
                {onViewOnGraph && f.nodeIds && f.nodeIds.length > 0 ? (
                  <Button size="xs" variant="outline" onClick={() => onViewOnGraph(f)}>{translate(`${T}viewOnGraph`, 'View on graph')}</Button>
                ) : null}
              </li>
            ))}
          </ul>
        </section>
      ))}
      <ImpactEvidenceSheet findingId={evidenceId} title={findings.find((f) => f.id === evidenceId)?.title} onClose={() => setEvidenceId(null)} />
    </div>
  )
}
