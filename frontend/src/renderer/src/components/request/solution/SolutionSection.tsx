/**
 * SolutionSection — CR-REQ-020-02
 *
 * Titled block shared by Diagnosis/Findings/Answer views. Empty sections show
 * a "No data" placeholder instead of throwing on missing fields.
 *
 * @module components/request/solution/SolutionSection
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import { RequestMarkdownContent } from './RequestMarkdownContent'

export function SolutionSection({
  title,
  lines,
  asList = false
}: {
  title: string
  lines: string[] | undefined
  asList?: boolean
}): React.JSX.Element {
  const has = lines && lines.length > 0
  return (
    <section className="space-y-1.5">
      <h3 className="text-sm font-medium">{title}</h3>
      {!has ? (
        <p className="text-sm text-muted-foreground">
          {translate('auto.components.request.SolutionPanel.noData', 'No data')}
        </p>
      ) : asList ? (
        <ul className="list-disc space-y-1 pl-5 text-sm">
          {lines.map((line, i) => (
            <li key={i}>{line}</li>
          ))}
        </ul>
      ) : (
        <RequestMarkdownContent content={lines.join('\n\n')} title={title} />
      )}
    </section>
  )
}
