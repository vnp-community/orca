/**
 * OutcomeRequirementsTable — FE-OR-T205
 *
 * Every acceptance criterion with where it stood before the work and where it stands after. "Delivered" here means the tasks the
 * plan tied to the criterion are done; the detail says so. A finding that a criterion regressed comes from an agent and is
 * labelled that way.
 *
 * @module components/request/outcome-review/diff/OutcomeRequirementsTable
 */

import React from 'react'
import { Badge } from '@/components/ui/badge'
import { agentReportedText, unverifiedText } from '../outcome-review-copy'
import {
  emptyText,
  flagText,
  requirementColumnText,
  requirementStateText
} from './outcome-diff-copy'
import { OutcomeEvidenceChip } from './OutcomeEvidenceChip'
import { rowOf } from './outcome-diff-view-model'
import type { ReviewItem } from '../../../../../../shared/outcome-review-types'

export function OutcomeRequirementsTable({ items }: { items: ReviewItem[] }): React.JSX.Element {
  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">{emptyText()}</p>
  }
  return (
    <table className="w-full text-sm" data-testid="outcome-requirements-table">
      <thead>
        <tr className="text-left text-xs text-muted-foreground">
          <th scope="col" className="py-1 pr-3 font-medium">
            {requirementColumnText('requirement')}
          </th>
          <th scope="col" className="py-1 pr-3 font-medium">
            {requirementColumnText('before')}
          </th>
          <th scope="col" className="py-1 pr-3 font-medium">
            {requirementColumnText('after')}
          </th>
          <th scope="col" className="py-1 font-medium">
            {requirementColumnText('evidence')}
          </th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => {
          const row = rowOf(item)
          return (
            <tr
              key={item.key}
              className="border-t border-border/60 align-top"
              data-testid="outcome-requirement-row"
              data-after={item.after}
            >
              <td className="py-2 pr-3">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{item.title}</span>
                  {row.agentReported && <Badge variant="outline">{agentReportedText()}</Badge>}
                  {item.severity === 'regression' && (
                    <Badge variant="destructive">{flagText('regression')}</Badge>
                  )}
                  {item.unverified && <Badge variant="outline">{unverifiedText()}</Badge>}
                </div>
                {item.detail && <p className="text-xs text-muted-foreground">{item.detail}</p>}
              </td>
              <td className="py-2 pr-3">{requirementStateText(item.before)}</td>
              <td className="py-2 pr-3 font-medium">{requirementStateText(item.after)}</td>
              <td className="py-2">
                <div className="flex flex-wrap gap-1">
                  {item.evidence.map((e, i) => (
                    <OutcomeEvidenceChip key={`${e.kind}:${e.ref}:${i}`} evidence={e} />
                  ))}
                </div>
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}
