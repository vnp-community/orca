/**
 * AnswerView — CR-REQ-020-02, SOL-FE-E2E-005
 *
 * @module components/request/solution/AnswerView
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { Solution } from '../../../../../shared/request-types'
import { RequestMarkdownContent } from './RequestMarkdownContent'
import { readAnswerDocument, type AnswerCitation } from './solution-answer-document'
import { SolutionSection } from './SolutionSection'

function PlainListSection({
  title,
  items,
  testId
}: {
  title: string
  items: string[]
  testId: string
}): React.JSX.Element | null {
  if (items.length === 0) {
    return null
  }
  return (
    <section className="space-y-1.5" data-testid={testId}>
      <h3 className="text-sm font-medium">{title}</h3>
      <ul className="list-disc space-y-1 pl-5 text-sm">
        {items.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </section>
  )
}

function CitationsSection({ citations }: { citations: AnswerCitation[] }): React.JSX.Element {
  return (
    <section className="space-y-1.5" data-testid="answer-citations">
      <h3 className="text-sm font-medium">
        {translate('auto.components.request.AnswerView.citations', 'Citations')}
      </h3>
      {citations.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {translate('auto.components.request.SolutionPanel.noData', 'No data')}
        </p>
      ) : (
        <ul className="list-disc space-y-1 pl-5 text-sm">
          {citations.map((citation, i) => (
            <li key={i} className="break-words">
              <span className="font-mono text-xs">{citation.ref}</span>
              {citation.excerpt ? (
                <span className="block text-xs text-muted-foreground">{citation.excerpt}</span>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export function AnswerView({ solution }: { solution: Solution }): React.JSX.Element {
  const doc = readAnswerDocument(solution.content)
  if (doc.rawText !== null) {
    return (
      <div data-testid="answer-view" className="space-y-4">
        <RequestMarkdownContent content={doc.rawText} />
      </div>
    )
  }
  return (
    <div data-testid="answer-view" className="space-y-4">
      <SolutionSection
        title={translate('auto.components.request.AnswerView.answer', 'Answer')}
        lines={doc.answerMarkdown ? [doc.answerMarkdown] : undefined}
      />
      {doc.confidencePercent !== null ? (
        <p className="text-xs text-muted-foreground" data-testid="answer-confidence">
          {translate('auto.components.request.AnswerView.confidence', 'Confidence: {{percent}}%', {
            percent: doc.confidencePercent
          })}
        </p>
      ) : null}
      <CitationsSection citations={doc.citations} />
      <PlainListSection
        title={translate('auto.components.request.AnswerView.assumptions', 'Assumptions')}
        items={doc.assumptions}
        testId="answer-assumptions"
      />
      <PlainListSection
        title={translate('auto.components.request.AnswerView.limitations', 'Limitations')}
        items={doc.limitations}
        testId="answer-limitations"
      />
      {doc.followUp ? (
        <section className="space-y-1.5" data-testid="answer-follow-up">
          <h3 className="text-sm font-medium">
            {translate('auto.components.request.AnswerView.followUp', 'Suggested follow-up')}
          </h3>
          <p className="text-sm">
            {doc.followUp.type
              ? translate(
                  'auto.components.request.AnswerView.followUpWithType',
                  '{{title}} ({{type}})',
                  { title: doc.followUp.title, type: doc.followUp.type }
                )
              : doc.followUp.title}
          </p>
        </section>
      ) : null}
    </div>
  )
}
