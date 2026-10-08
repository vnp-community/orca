/**
 * ExecutionChecksTable — FE-REQ-TASK-036-08
 *
 * @module components/request/execution/ExecutionChecksTable
 */

import React from 'react'
import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { CheckComparison } from './execution-result-comparison'

const T = 'auto.components.request.execution.'

export function ExecutionChecksTable({ rows }: { rows: readonly CheckComparison[] }): React.JSX.Element | null {
  if (rows.length === 0) {return null}
  return (
    <div className="flex flex-col gap-1" data-testid="execution-checks">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{translate(`${T}checks.check`, 'Check')}</TableHead>
            <TableHead>{translate(`${T}checks.agent`, 'Agent reported')}</TableHead>
            <TableHead>{translate(`${T}checks.orca`, 'Orca re-ran')}</TableHead>
            <TableHead>{translate(`${T}checks.compare`, 'Comparison')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.id}>
              <TableCell className="font-mono text-xs">{r.id}</TableCell>
              <TableCell className="text-xs">{r.agentExit === null ? '-' : `exit ${r.agentExit}`}</TableCell>
              <TableCell className="text-xs">
                {r.orcaPassed === null
                  ? translate(`${T}checks.notRerun`, 'Not re-run yet')
                  : r.orcaPassed
                    ? translate(`${T}checks.passed`, 'Passed')
                    : translate(`${T}checks.failed`, 'Failed')}
              </TableCell>
              <TableCell className="text-xs">
                {r.mismatch ? (
                  <span className="inline-flex items-center gap-1 text-risk-medium">
                    <TriangleAlert className="size-3" aria-hidden />
                    {translate(`${T}checks.mismatch`, 'Mismatch')}
                  </span>
                ) : null}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <p className="text-[11px] text-muted-foreground">{translate(`${T}checks.note`, 'Orca does not rely on the agent’s word.')}</p>
    </div>
  )
}
