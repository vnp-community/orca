/**
 * SolutionDimensionTable — FE-REQ-TASK-036-05
 *
 * Option x impact dimension. The recommended column is labelled, never preselected.
 *
 * @module components/request/impact/SolutionDimensionTable
 */

import React from 'react'
import { ArrowLeftRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { RiskBadge } from '../../graph/RiskBadge'
import { buildComparisonRows } from './impact-dimension-model'
import type { ImpactComparison } from '../../../../../shared/request-artifact-types'

const T = 'auto.components.request.impact.'

type Props = {
  options: { id: string; title: string }[]
  comparison: readonly ImpactComparison[] | null
  recommendedId?: string
}

export function SolutionDimensionTable({ options, comparison, recommendedId }: Props): React.JSX.Element | null {
  const rows = buildComparisonRows(options.map((o) => o.id), comparison)
  if (rows.length === 0) {return null}
  const notAssessed = translate(`${T}notAssessed`, 'Not assessed')
  return (
    <div className="overflow-x-auto scrollbar-sleek" data-testid="solution-dimension-table">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="sticky left-0 bg-background">{translate(`${T}dimension`, 'Dimension')}</TableHead>
            {options.map((o) => (
              <TableHead key={o.id}>
                {o.title}
                {o.id === recommendedId ? <span className="ml-1 text-primary">({translate(`${T}recommended`, 'Recommended')})</span> : null}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.dimension}>
              <TableHead scope="row" className="sticky left-0 whitespace-nowrap bg-background">
                <span className="inline-flex items-center gap-1">
                  {translate(`${T}dim.${row.dimension}`, row.dimension)}
                  {row.differs ? (
                    <>
                      <ArrowLeftRight className="size-3 text-muted-foreground" aria-hidden />
                      <span className="sr-only">{translate(`${T}differs`, 'Differs between options')}</span>
                    </>
                  ) : null}
                </span>
              </TableHead>
              {options.map((o) => {
                const cell = row.cells[o.id]
                return (
                  <TableCell key={o.id}>
                    {cell ? (
                      <div className="flex flex-col gap-0.5">
                        <span className="flex items-center gap-1">
                          <RiskBadge level={cell.level} size="sm" />
                          {cell.score !== null ? <span className="text-xs tabular-nums">{cell.score}</span> : null}
                        </span>
                        {cell.note ? <span className="text-xs text-muted-foreground">{cell.note}</span> : null}
                      </div>
                    ) : (
                      <span className="text-xs text-muted-foreground">{notAssessed}</span>
                    )}
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
