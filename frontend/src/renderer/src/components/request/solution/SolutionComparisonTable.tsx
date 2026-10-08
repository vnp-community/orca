/**
 * SolutionComparisonTable — CR-REQ-020-03
 *
 * Criteria as rows, options as columns; rows whose cells differ get an icon.
 *
 * @module components/request/solution/SolutionComparisonTable
 */

import React from 'react'
import { ArrowLeftRight } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { SolutionOption } from '../../../../../shared/request-types'
import { buildComparisonRows } from './solution-view-model'

const CRITERION_FALLBACK: Record<string, string> = {
  summary: 'Summary',
  pros: 'Pros',
  cons: 'Cons',
  effort: 'Effort',
  risk: 'Risk'
}

export function SolutionComparisonTable({ options }: { options: SolutionOption[] }): React.JSX.Element {
  const rows = buildComparisonRows(options)
  const noData = translate('auto.components.request.SolutionPanel.noData', 'No data')
  const differs = translate('auto.components.request.SolutionComparisonTable.differs', 'Differs between options')

  return (
    <div className="overflow-x-auto scrollbar-sleek" data-testid="solution-comparison-table">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>
              {translate('auto.components.request.SolutionComparisonTable.criterion', 'Criterion')}
            </TableHead>
            {options.map((o) => (
              <TableHead key={o.id}>{o.title || noData}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.criterion}>
              <TableHead scope="row" className="whitespace-nowrap">
                <span className="inline-flex items-center gap-1">
                  {translate(
                    `auto.components.request.SolutionComparisonTable.row.${row.criterion}`,
                    CRITERION_FALLBACK[row.criterion]
                  )}
                  {row.differs && <ArrowLeftRight className="size-3" role="img" aria-label={differs} />}
                </span>
              </TableHead>
              {row.cells.map((cell, i) => (
                <TableCell key={options[i].id} className="whitespace-normal align-top">
                  {cell || <span className="text-muted-foreground">{noData}</span>}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
