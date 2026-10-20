/**
 * OutcomeVerdictCards — FE-OR-T122
 *
 * Requirements, Architecture, Quality gate and Release risk of a delivered review. Each card states only what its data supports:
 * "Every task tied to these criteria is done" is not "requirements met"; a gate or a risk nobody measured is "not enough data",
 * never a pass or a low.
 *
 * @module components/request/outcome-review/OutcomeVerdictCards
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import {
  agentReportedText,
  gateText,
  riskText,
  sectionText,
  unknownText
} from './outcome-review-copy'
import { gateOf, requirementsCardOf, riskOf } from './outcome-review-view-model'
import type { OutcomeReview } from '../../../../../shared/outcome-review-types'

const T = 'auto.components.request.outcomeReview.OutcomeVerdictCards.'

function Tile({
  id,
  title,
  children
}: {
  id: string
  title: string
  children: React.ReactNode
}): React.JSX.Element {
  return (
    <section
      className="flex min-w-0 flex-col gap-1 rounded-md border border-border p-3"
      aria-label={title}
      data-testid={`outcome-verdict-${id}`}
    >
      <h4 className="text-xs font-medium text-muted-foreground">{title}</h4>
      {children}
    </section>
  )
}

function flagText(flag: string): string {
  switch (flag) {
    case 'migration':
      return translate(`${T}flag.migration`, 'Migration')
    case 'breaking':
      return translate(`${T}flag.breaking`, 'Breaking change')
    case 'config':
      return translate(`${T}flag.config`, 'Config change')
    default:
      return unknownText()
  }
}

export function OutcomeVerdictCards({ review }: { review: OutcomeReview }): React.JSX.Element {
  const req = requirementsCardOf(review)
  const arch = review.model.verdict.architecture
  const { risk, flags } = riskOf(review)
  return (
    <div
      className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4"
      data-testid="outcome-verdict-cards"
    >
      <Tile id="requirements" title={sectionText('requirements')}>
        {req.noData ? (
          <p className="text-sm">
            {translate(`${T}requirements.noData`, 'Not enough data to conclude')}
          </p>
        ) : (
          <>
            <p className="text-lg font-semibold tabular-nums">
              {translate(`${T}requirements.count`, '{{delivered}} of {{total}}', {
                delivered: req.delivered,
                total: req.total
              })}
            </p>
            <p className="text-xs text-muted-foreground">
              {req.allDelivered
                ? translate(`${T}requirements.all`, 'Every task tied to these criteria is done')
                : translate(
                    `${T}requirements.rest`,
                    '{{partial}} partial · {{notCovered}} not covered · {{unknown}} unknown',
                    {
                      partial: req.partial,
                      notCovered: req.notCovered,
                      unknown: req.unknown
                    }
                  )}
            </p>
          </>
        )}
      </Tile>
      <Tile id="architecture" title={sectionText('architecture')}>
        {arch.designChecked ? (
          <p className="text-lg font-semibold tabular-nums">
            {translate(`${T}architecture.deviations`, '{{count}} reported deviations', {
              count: arch.reportedDeviations
            })}
          </p>
        ) : (
          <p className="text-sm">{unknownText()}</p>
        )}
        <Badge variant="outline" className="w-fit">
          {agentReportedText()}
        </Badge>
      </Tile>
      <Tile id="quality" title={translate(`${T}quality.title`, 'Quality gate')}>
        <p className="text-sm">{gateText(gateOf(review))}</p>
      </Tile>
      <Tile id="release" title={translate(`${T}release.title`, 'Release risk')}>
        <p className="text-sm">{riskText(risk)}</p>
        {flags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {flags.map((f) => (
              <Badge key={f} variant="secondary">
                {flagText(f)}
              </Badge>
            ))}
          </div>
        )}
      </Tile>
    </div>
  )
}
