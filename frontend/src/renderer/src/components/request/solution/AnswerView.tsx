/**
 * AnswerView — CR-REQ-020-02
 *
 * @module components/request/solution/AnswerView
 */

import React from 'react'
import { translate } from '@/i18n/i18n'
import type { Solution } from '../../../../../shared/request-types'
import { RequestMarkdownContent } from './RequestMarkdownContent'
import { readContentSections } from './solution-content-sections'
import { SolutionSection } from './SolutionSection'

const SPECS = [
  { name: 'answer', aliases: ['answer', 'response'] },
  { name: 'citations', aliases: ['citations', 'sources', 'references'] }
]

export function AnswerView({ solution }: { solution: Solution }): React.JSX.Element {
  const { sections, fallbackText } = readContentSections(solution.content, SPECS)
  return (
    <div data-testid="answer-view" className="space-y-4">
      {fallbackText !== null ? (
        <RequestMarkdownContent content={fallbackText} />
      ) : (
        <>
          <SolutionSection
            title={translate('auto.components.request.AnswerView.answer', 'Answer')}
            lines={sections.answer}
          />
          <SolutionSection
            title={translate('auto.components.request.AnswerView.citations', 'Citations')}
            lines={sections.citations}
            asList
          />
        </>
      )}
    </div>
  )
}
